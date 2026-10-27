package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	DataDir string
}

type Job struct {
	VideoID string
	Quality string
	Height  int
}

func (c *Config) Convert(job Job, sourcePath string) error {
	output := filepath.Join(c.DataDir, job.VideoID, job.Quality+".mp4")
	// min(target, source) avoids upscaling small uploads; -2 keeps width divisible by two.
	filter := fmt.Sprintf("scale=-2:'min(%d,ih)'", job.Height)
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", sourcePath,
		"-map", "0:v:0", "-map", "0:a:0?", "-vf", filter,
		"-c:v", "libx264", "-preset", "medium", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", output)
	result, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(output)
		message := strings.TrimSpace(string(result))
		if len(message) > 500 {
			message = message[len(message)-500:]
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, message)
	}
	return nil
}
