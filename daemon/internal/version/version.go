package version

// Version is the daemon version. Defaults to a dev build; release builds
// override it via -ldflags, e.g.
//
//	go build -ldflags "-X envorca.dev/envorca/daemon/internal/version.Version=0.2.0" ./cmd/envorcad
var Version = "0.1.0-dev"