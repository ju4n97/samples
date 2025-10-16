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

	"github.com/ekisa-team/syn4pse/config"
)

const (
	defaultRetryDelay = 2 * time.Second
	defaultMaxRetries = 3
	defaultTimeout    = 5 * time.Minute
)

// HuggingFaceDownloader downloads a model from Hugging Face.
type HuggingFaceDownloader struct{}

// Download downloads Hugging Face model to local cache and returns the actual model file path.
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

			modelPath, err := resolveModelPath(fullPath, hfSource.Include)
			if err != nil {
				return "", false, fmt.Errorf("failed to resolve model path: %w", err)
			}

			return modelPath, false, nil
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

// resolveModelPath finds the actual model file based on include patterns.
// If no include patterns or multiple files match, returns the base directory.
// If a single specific file is matched, returns that file path.
func resolveModelPath(baseDir string, includePatterns []string) (string, error) {
	// If no include patterns, return the directory
	if len(includePatterns) == 0 {
		return baseDir, nil
	}

	// Collect all matching files
	var allMatches []string
	for _, pattern := range includePatterns {
		fullPattern := filepath.Join(baseDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			slog.Warn("Invalid glob pattern", "pattern", pattern, "error", err)
			continue
		}

		allMatches = append(allMatches, matches...)
	}

	if len(allMatches) == 0 {
		slog.Warn("No files matched include patterns, using base directory", "patterns", includePatterns)
		return baseDir, nil
	}

	// Filter out directories, keep only files
	var fileMatches []string
	for _, match := range allMatches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			fileMatches = append(fileMatches, match)
		}
	}

	if len(fileMatches) == 0 {
		// Only directories matched, return base directory
		return baseDir, nil
	}

	if len(fileMatches) == 1 {
		// Single file matched,this is the model file
		slog.Info("Resolved model file", "path", fileMatches[0])
		return fileMatches[0], nil
	}

	// Multiple files matched, try to find the primary model file
	modelFile := findPrimaryModelFile(fileMatches)
	if modelFile != "" {
		slog.Info("Resolved primary model file from multiple matches", "path", modelFile, "total_matches", len(fileMatches))
		return modelFile, nil
	}

	// Can't determine primary file, return base directory and let backend handle it
	slog.Warn("Multiple files matched, using base directory", "count", len(fileMatches), "files", fileMatches)
	return baseDir, nil
}

// findPrimaryModelFile attempts to identify the primary model file from multiple matches.
// It looks for common model file extensions and patterns.
func findPrimaryModelFile(files []string) string {
	// Priority order for model file extensions
	extensions := []string{
		".onnx",        // ONNX models (Piper, etc.)
		".bin",         // Binary models (Whisper, etc.)
		".gguf",        // GGUF models (llama.cpp)
		".safetensors", // SafeTensors
		".pt",          // PyTorch
		".pth",         // PyTorch
		".pkl",         // Pickle
		".h5",          // HDF5 (Keras)
	}

	// First pass: look for files with priority extensions
	for _, ext := range extensions {
		for _, file := range files {
			if strings.HasSuffix(strings.ToLower(file), ext) {
				return file
			}
		}
	}

	// Second pass: look for specific patterns indicating primary model
	patterns := []string{"model", "checkpoint", "weights"}
	for _, pattern := range patterns {
		for _, file := range files {
			baseName := strings.ToLower(filepath.Base(file))
			if strings.Contains(baseName, pattern) {
				return file
			}
		}
	}

	// No clear primary file found
	return ""
}
