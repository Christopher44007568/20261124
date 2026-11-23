package optics

import (
	"fmt"
	"os"
	"sync"
)

// Manager owns the currently active ("last known good") configuration.
// A rejected candidate never displaces the active config, so the renderer
// keeps showing the last usable frame.
type Manager struct {
	mu     sync.RWMutex
	active *Config
	path   string
}

// NewManager creates an empty manager bound to a persistence path.
func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// Active returns the last known good config, or nil if none was ever accepted.
func (m *Manager) Active() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// Apply validates a candidate config and promotes it only when it passes
// field validation and the roundtrip mapping check. On failure the active
// config is untouched and the error names every offending field.
func (m *Manager) Apply(candidate *Config) error {
	if err := Validate(candidate); err != nil {
		return fmt.Errorf("keeping last good frame: %w", err)
	}
	if _, err := CheckRoundtrip(candidate, 9); err != nil {
		return fmt.Errorf("keeping last good frame: %w", err)
	}
	m.mu.Lock()
	m.active = candidate
	m.mu.Unlock()
	return nil
}

// ApplyJSON decodes and applies a raw JSON payload.
func (m *Manager) ApplyJSON(data []byte) error {
	cfg, err := Decode(data)
	if err != nil {
		return fmt.Errorf("keeping last good frame: %w", err)
	}
	return m.Apply(cfg)
}

// Save persists the active config to the manager's path.
func (m *Manager) Save() error {
	m.mu.RLock()
	cfg := m.active
	m.mu.RUnlock()
	if cfg == nil {
		return fmt.Errorf("no active config to save")
	}
	data, err := Encode(cfg)
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// Reload loads the config file from disk and applies it. If the file is
// missing, corrupt, or invalid, the active config stays in place.
func (m *Manager) Reload() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("keeping last good frame: cannot read %s: %w", m.path, err)
	}
	return m.ApplyJSON(data)
}
