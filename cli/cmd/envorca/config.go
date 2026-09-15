package main

import (
	"os"

	"gopkg.in/yaml.v3"

	"envorca.dev/envorca/api/ipc"
)

// configFile is the CLI-wide --config flag. The daemon is the only owner of
// configuration semantics; the CLI reads only the IPC endpoint so clients
// and daemon resolve the same socket (configuration.md).
var configFile string

// configPath returns the config file the user selected: the --config flag
// when given, otherwise ENVORCA_CONFIG. The empty string means the daemon's
// platform defaults apply.
func configPath() string {
	if configFile != "" {
		return configFile
	}
	return os.Getenv("ENVORCA_CONFIG")
}

// commandEndpoint resolves the IPC endpoint the same way the daemon does:
// the socket setting from the config file, then ENVORCA_SOCKET, then the
// platform default (ADR-0003).
func commandEndpoint() string {
	if p := configPath(); p != "" {
		b, err := os.ReadFile(p)
		if err == nil {
			var cfg struct {
				Socket string `yaml:"socket"`
			}
			if yaml.Unmarshal(b, &cfg) == nil && cfg.Socket != "" {
				return cfg.Socket
			}
		}
	}
	return ipc.DefaultEndpoint(ipc.DefaultStateDir())
}