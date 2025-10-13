package config

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches for configuration changes.
type Watcher struct {
	path       string
	schemaPath string
	onReload   func(*Config, error)
	current    *Config
	mu         sync.RWMutex
	reloads    atomic.Uint32
}

// NewWatcher creates a new config watcher.
func NewWatcher(path string, schemaPath string, onReload func(*Config, error)) (*Watcher, error) {
	watcher := &Watcher{
		path:       path,
		schemaPath: schemaPath,
		onReload:   onReload,
	}

	cfg, err := LoadAndValidate(path, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}
	watcher.current = cfg

	go watcher.watch()

	return watcher, nil
}

// watch watches for configuration changes.
func (cw *Watcher) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to create file watcher", "error", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(cw.path); err != nil {
		slog.Error("Failed to watch config file", "path", cw.path, "error", err)
		return
	}

	var timer *time.Timer
	const debounce = 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				if timer != nil {
					timer.Stop()
				}

				timer = time.AfterFunc(debounce, func() {
					cw.reload()
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			slog.Error("Watcher error", "error", err)
		}
	}
}

// reload reloads the config file.
func (cw *Watcher) reload() {
	count := cw.reloads.Add(1)
	slog.Info("Reloading config file", "path", cw.path, "count", count)

	cfg, err := LoadAndValidate(cw.path, cw.schemaPath)
	if err != nil {
		slog.Error("Failed to reload config", "error", err)
		cw.onReload(nil, err)
		return
	}

	cw.mu.Lock()
	cw.current = cfg
	cw.mu.Unlock()

	slog.Info("Config reloaded successfully", "count", count)
	cw.onReload(cfg, nil)
}

// Snapshot returns the current config snapshot (thread-safe).
func (cw *Watcher) Snapshot() *Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	return cw.current
}

// ReloadCount returns the number of times the config has been reloaded.
func (cw *Watcher) ReloadCount() uint32 {
	return cw.reloads.Load()
}
