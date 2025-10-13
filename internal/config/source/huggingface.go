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
	markerFilename    = ".syn4pse-downloaded"
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
	markerPath := filepath.Join(fullPath, markerFilename)
	markerContent := d.markerContent(repo, hfSource.Revision)

	if _, err := os.Stat(markerPath); err == nil {
		if !d.shouldRedownload(markerPath, markerContent) {
			slog.Info("Model already downloaded and up-to-date (marker match), skipping", "repo", repo, "path", fullPath)
			return fullPath, true, nil
		}
	}

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
			if err := os.WriteFile(markerPath, []byte(markerContent), 0o644); err != nil {
				slog.Warn("Failed to write download marker", "path", markerPath, "error", err)
			} else {
				slog.Info("Download marker updated", "path", markerPath)
			}

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

// markerContent generates the expected content of the marker file.
// Used to detect if we need to redownload due to config change.
func (d *HuggingFaceDownloader) markerContent(repo, revision string) string {
	return fmt.Sprintf("repo: %s\nrevision: %s\n", repo, revision)
}

// shouldRedownload checks if the model should be redownloaded by comparing marker content.
func (d *HuggingFaceDownloader) shouldRedownload(markerPath, expectedContent string) bool {
	content, err := os.ReadFile(markerPath)
	if err != nil {
		slog.Debug("Marker file missing or unreadable", "path", markerPath, "error", err)
		return true
	}

	if string(content) != expectedContent {
		slog.Info("Model config changed (marker mismatch), will redownload",
			"marker_path", markerPath,
			"expected_snippet", expectedContent,
			"actual_snippet", string(content))
		return true
	}

	return false
}
