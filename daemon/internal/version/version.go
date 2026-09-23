package version

import (
	"runtime/debug"
	"strings"
)

// Version is the daemon version. Defaults to a dev build; release builds
// override it via -ldflags, e.g.
//
//	go build -ldflags "-X envorca.dev/envorca/daemon/internal/version.Version=0.2.0" ./cmd/envorcad
var Version = "0.1.0-dev"

// Commit is the git commit the daemon was built from. Make and release
// builds override it via -ldflags; every other build falls back to the VCS
// revision the Go toolchain embeds automatically when compiling from a git
// checkout.
var Commit string

// ShortCommit returns a short (7-char) build identifier: the ldflags-injected
// Commit when present, the embedded VCS revision otherwise, or "".
func ShortCommit() string {
	c := Commit
	if c == "" {
		c = vcsSetting("vcs.revision")
	}
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

// Manifested reports whether the build was produced from a dirty git tree.
func Manifested() bool {
	return vcsSetting("vcs.modified") == "true"
}

// Full returns the version with its build commit, e.g. "0.1.0-dev (7307870)"
// or "0.1.0-dev (7307870-dirty)" when built from uncommitted changes.
func Full() string {
	if c := ShortCommit(); c != "" {
		if Manifested() {
			c += "-dirty"
		}
		return Version + " (" + c + ")"
	}
	return Version
}

func vcsSetting(key string) string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == key {
				return strings.TrimSpace(s.Value)
			}
		}
	}
	return ""
}