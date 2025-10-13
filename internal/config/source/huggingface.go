package source

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ekisa-team/syn4pse/internal/config"
)

const (
	defaultRetryDelay = 2 * time.Second
	defaultMaxRetries = 3
	defaultTimeout    = 5 * time.Minute
)

// HuggingFaceDownloader downloads a model from Hugging Face.
type HuggingFaceDownloader struct{}

// Download downloads Hugging Face model to local cache.
func (d *HuggingFaceDownloader) Download(ctx context.Context, modelConfig *config.ModelConfig, targetDir string) (string, bool, error) {
	source, err := modelConfig.GetSource()
	if err != nil {
		return "", false, fmt.Errorf("failed to get model source: %w", err)
	}

	hfSource, ok := source.(config.HuggingFaceSource)
	if !ok {
		return "", false, fmt.Errorf("invalid source type: %T", source)
	}

	repo := strings.TrimSpace(hfSource.Repo)
	if repo == "" {
		return "", false, fmt.Errorf("invalid repo name: %s", repo)
	}

	fullPath := filepath.Join(targetDir, repo)

	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		return "", false, fmt.Errorf("failed to create directory: %w", err)
	}

	args := []string{
		"download",
		repo,
		"--local-dir", fullPath,
	}

	if hfSource.Revision != "" {
		args = append(args, "--revision", hfSource.Revision)
	}
	if hfSource.RepoType != "" {
		args = append(args, "--repo-type", hfSource.RepoType)
	}
	for _, inc := range hfSource.Include {
		args = append(args, "--include", inc)
	}
	for _, exc := range hfSource.Exclude {
		args = append(args, "--exclude", exc)
	}
	if hfSource.ForceDownload {
		args = append(args, "--force-download")
	}
	if hfSource.Token != "" {
		args = append(args, "--token", hfSource.Token)
	}
	if hfSource.MaxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprintf("%d", hfSource.MaxWorkers))
	}

	var lastErr error
	for attempt := range defaultMaxRetries {
		if attempt > 0 {
			slog.Info("Retrying download", "repo", repo, "attempt", attempt+1, "last_error", lastErr)
			time.Sleep(defaultRetryDelay)
		} else {
			slog.Info("Downloading model", "repo", repo, "path", fullPath)
		}

		delayCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
		cmd := exec.CommandContext(delayCtx, "hf", args...)
		output, err := cmd.CombinedOutput()
		cancel()

		if err == nil {

			slog.Info("Model downloaded successfully", "repo", repo, "path", fullPath, "attempt", attempt+1)
			return fullPath, false, nil
		}

		lastErr = err
		slog.Error("Failed to download model", "repo", repo, "path", fullPath, "attempt", attempt+1, "error", err, "output", string(output))

		if delayCtx.Err() == context.DeadlineExceeded {
			slog.Warn("Download timed out", "repo", repo, "path", fullPath, "attempt", attempt+1)
		} else if delayCtx.Err() == context.Canceled {
			return "", false, fmt.Errorf("download canceled: %w", err)
		}
	}

	return "", false, lastErr
}
