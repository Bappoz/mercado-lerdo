// Package buildinfo reports the binary version, injected at link time via
// -ldflags and complemented by the VCS metadata the Go toolchain embeds.
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// version is overridden in release builds:
// -ldflags "-X github.com/Bappoz/software-engineering-simulator/internal/buildinfo.version=v0.1.0"
var version = "dev"

// Info describes the running binary.
type Info struct {
	Version  string
	Commit   string
	Modified bool
	Go       string
	Platform string
}

// Read collects build information for the current binary.
func Read() Info {
	info := Info{
		Version:  version,
		Commit:   "unknown",
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}

func (i Info) String() string {
	commit := i.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	if i.Modified {
		commit += "-dirty"
	}
	return fmt.Sprintf("sesim %s (%s, %s, %s)", i.Version, commit, i.Go, i.Platform)
}
