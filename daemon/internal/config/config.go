package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"runorka.dev/runorka/api/ipc"
)

// Config holds daemon settings. Empty fields resolve to platform defaults,
// so a missing config file is not an error.
type Config struct {
	// StateDir is the root of Runorka local application state.
	StateDir string `yaml:"state_dir"`
	// LogLevel is one of debug, info, warn, error.
	LogLevel string `yaml:"log_level"`
	// Socket overrides the IPC endpoint (pipe name on Windows, socket path
	// elsewhere). Env override RUNORKA_SOCKET wins if both are set.
	Socket string `yaml:"socket"`
}

// Defaults returns the platform default configuration.
func Defaults() Config {
	return Config{
		StateDir: ipc.DefaultStateDir(),
		LogLevel: "info",
	}
}

func (c Config) RuntimeDir() string { return filepath.Join(c.StateDir, "run") }
func (c Config) LogDir() string     { return filepath.Join(c.StateDir, "logs") }
func (c Config) DBPath() string     { return filepath.Join(c.StateDir, "state.db") }

// Endpoint returns the IPC endpoint for this configuration.
func (c Config) Endpoint() string {
	if c.Socket != "" {
		return c.Socket
	}
	return ipc.DefaultEndpoint(c.StateDir)
}

// Load reads the given YAML file, falling back to defaults when absent, and
// creates the state directories.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		path = os.Getenv("RUNORKA_CONFIG")
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
		if cfg.StateDir == "" {
			cfg.StateDir = Defaults().StateDir
		}
		if cfg.LogLevel == "" {
			cfg.LogLevel = "info"
		}
	}
	for _, dir := range []string{cfg.StateDir, cfg.RuntimeDir(), cfg.LogDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Config{}, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return cfg, nil
}