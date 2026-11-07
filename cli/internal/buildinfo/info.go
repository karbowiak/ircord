// Package buildinfo shares sanitized release information between the CLI and API.
package buildinfo

import (
	"runtime"
	"runtime/debug"
)

// Set with -ldflags -X in release builds. Development builds use VCS information.
var Version = "dev"
var Commit = ""

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"goVersion"`
}

func Current() Info {
	i := Info{Version: Version, Commit: Commit, GoVersion: runtime.Version()}
	if b, ok := debug.ReadBuildInfo(); ok {
		for _, s := range b.Settings {
			if s.Key == "vcs.revision" && i.Commit == "" {
				i.Commit = s.Value
			}
		}
	}
	if i.Commit == "" {
		i.Commit = "unknown"
	}
	return i
}
