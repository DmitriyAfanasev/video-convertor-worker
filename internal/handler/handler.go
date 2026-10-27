package handler

import (
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ffmpeg-learn/internal/config"
	"ffmpeg-learn/internal/ffmpeg"
	"ffmpeg-learn/internal/store"
	"ffmpeg-learn/internal/util"
)

type App struct {
	config  config.Config
	store   *store.Store
	jobs    chan ffmpeg.Job
	log     *slog.Logger
	ffmpeg  *ffmpeg.Config
}

func New(cfg config.Config, s *store.Store, jobs chan ffmpeg.Job, logger *slog.Logger) *App {
	return &App{
		config: cfg,
		store:  s,
		jobs:   jobs,
		log:    logger,
		ffmpeg: &ffmpeg.Config{DataDir: cfg.DataDir},
	}
}

func (a *App) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/videos", a.listVideos)
	mux.HandleFunc("POST /api/videos", a.uploadVideo)
	mux.HandleFunc("GET /api/videos/{id}", a.getVideo)
	mux.Handle("GET /media/", http.StripPrefix("/media/", http.FileServer(http.Dir(a.config.DataDir))))
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	util.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) listVideos(w http.ResponseWriter, _ *http.Request) {
	util.WriteJSON(w, http.StatusOK, a.store.List())
}

func (a *App) getVideo(w http.ResponseWriter, r *http.Request) {
	video, ok := a.store.Get(r.PathValue("id"))
	if !ok {
		util.WriteError(w, http.StatusNotFound, "video not found")
		return
	}
	util.WriteJSON(w, http.StatusOK, video)
}

func (a *App) uploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.config.MaxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid upload or file is too large")
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "multipart field 'video' is required")
		return
	}
	defer file.Close()

	id, err := util.RandomID()
	if err != nil {
		util.WriteError(w, http.StatusInternalServerError, "cannot create video id")
		return
	}
	dir := filepath.Join(a.config.DataDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		util.WriteError(w, http.StatusInternalServerError, "cannot create storage directory")
		return
	}
	sourcePath := filepath.Join(dir, "source"+safeExtension(header))
	if err := util.SaveUpload(sourcePath, file); err != nil {
		util.WriteError(w, http.StatusInternalServerError, "cannot save video")
		return
	}

	profiles := []struct {
		name   string
		height int
	}{{"1080p", 1080}, {"720p", 720}, {"480p", 480}, {"360p", 360}}
	video := store.Video{
		ID: id, Name: filepath.Base(header.Filename), CreatedAt: time.Now().UTC(),
		Status: "queued", SourceURL: "/media/" + id + "/" + filepath.Base(sourcePath),
	}
	for _, profile := range profiles {
		video.Renditions = append(video.Renditions, store.Rendition{Quality: profile.name, Status: "queued"})
	}
	if err := a.store.Add(video); err != nil {
		util.WriteError(w, http.StatusInternalServerError, "cannot save metadata")
		return
	}
	for _, profile := range profiles {
		a.jobs <- ffmpeg.Job{VideoID: id, Quality: profile.name, Height: profile.height}
	}
	util.WriteJSON(w, http.StatusAccepted, video)
}

func (a *App) Worker(number int) {
	for job := range a.jobs {
		a.log.Info("conversion started", "worker", number, "video", job.VideoID, "quality", job.Quality)
		_ = a.store.Update(job.VideoID, func(video *store.Video) {
			video.Status = "processing"
			setRendition(video, job.Quality, "processing", "", "")
		})
		video, _ := a.store.Get(job.VideoID)
		sourcePath := filepath.Join(a.config.DataDir, job.VideoID, filepath.Base(video.SourceURL))
		err := a.ffmpeg.Convert(job, sourcePath)
		_ = a.store.Update(job.VideoID, func(video *store.Video) {
			if err != nil {
				setRendition(video, job.Quality, "failed", "", err.Error())
			} else {
				url := "/media/" + job.VideoID + "/" + job.Quality + ".mp4"
				setRendition(video, job.Quality, "ready", url, "")
			}
			video.Status = aggregateStatus(video.Renditions)
		})
		if err != nil {
			a.log.Error("conversion failed", "worker", number, "video", job.VideoID, "quality", job.Quality, "error", err)
		} else {
			a.log.Info("conversion finished", "worker", number, "video", job.VideoID, "quality", job.Quality)
		}
	}
}

func setRendition(video *store.Video, quality, status, url, errorMessage string) {
	for i := range video.Renditions {
		if video.Renditions[i].Quality == quality {
			video.Renditions[i].Status = status
			video.Renditions[i].URL = url
			video.Renditions[i].Error = errorMessage
			return
		}
	}
}

func aggregateStatus(renditions []store.Rendition) string {
	ready, finished := 0, 0
	for _, rendition := range renditions {
		if rendition.Status == "ready" {
			ready++
		}
		if rendition.Status == "ready" || rendition.Status == "failed" {
			finished++
		}
	}
	if finished < len(renditions) {
		return "processing"
	}
	if ready == 0 {
		return "failed"
	}
	return "ready"
}

func safeExtension(header *multipart.FileHeader) string {
	ext := util.SafeExtension(header)
	return ext
}
