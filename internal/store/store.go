package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Rendition struct {
	Quality string `json:"quality"`
	URL     string `json:"url,omitempty"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

type Video struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"createdAt"`
	Status     string    `json:"status"`
	SourceURL  string    `json:"sourceUrl"`
	Renditions []Rendition `json:"renditions"`
}

type Store struct {
	mu      sync.RWMutex
	videos  map[string]Video
	dataDir string
}

func New(dataDir string) (*Store, error) {
	s := &Store{videos: make(map[string]Video), dataDir: dataDir}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dataDir, entry.Name(), "metadata.json"))
		if err != nil {
			continue
		}
		var video Video
		if json.Unmarshal(data, &video) == nil {
			// Jobs are in-memory in this demo. Mark interrupted work explicitly.
			if video.Status == "processing" || video.Status == "queued" {
				video.Status = "failed"
				for i := range video.Renditions {
					if video.Renditions[i].Status != "ready" {
						video.Renditions[i].Status = "failed"
						video.Renditions[i].Error = "server restarted during conversion"
					}
				}
			}
			s.videos[video.ID] = video
		}
	}
	return s, nil
}

func (s *Store) Add(video Video) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos[video.ID] = video
	return s.saveLocked(video)
}

func (s *Store) Get(id string) (Video, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	video, ok := s.videos[id]
	return video, ok
}

func (s *Store) List() []Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	videos := make([]Video, 0, len(s.videos))
	for _, video := range s.videos {
		videos = append(videos, video)
	}
	for i := 0; i < len(videos); i++ {
		for j := i + 1; j < len(videos); j++ {
			if videos[j].CreatedAt.After(videos[i].CreatedAt) {
				videos[i], videos[j] = videos[j], videos[i]
			}
		}
	}
	return videos
}

func (s *Store) Update(id string, update func(*Video)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	video, ok := s.videos[id]
	if !ok {
		return errors.New("video not found")
	}
	update(&video)
	s.videos[id] = video
	return s.saveLocked(video)
}

func (s *Store) saveLocked(video Video) error {
	data, err := json.MarshalIndent(video, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dataDir, video.ID, "metadata.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
