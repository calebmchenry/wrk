// Package buildinfo defines the shared release metadata and asset contract.
package buildinfo

import (
	"fmt"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// Set by release packaging through -ldflags -X. Plain go builds stay development builds.
var (
	Version = "dev"
	Commit  = "unknown"
	Kind    = "development"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildKind string `json:"build_kind"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
}

func Current() Info {
	i := Info{Version, Commit, Kind, runtime.GOOS, runtime.GOARCH}
	if i.Commit == "unknown" {
		if b, ok := debug.ReadBuildInfo(); ok {
			for _, s := range b.Settings {
				if s.Key == "vcs.revision" {
					i.Commit = s.Value
				}
			}
		}
	}
	return i
}

var stable = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var commit = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

func Stable(v string) bool { return stable.MatchString(v) }
func (i Info) IsRelease() bool {
	return i.BuildKind == "release" && Stable(i.Version) && commit.MatchString(i.Commit)
}

func FromTag(tag string) (string, error) {
	if !strings.HasPrefix(tag, "v") || !Stable(tag[1:]) {
		return "", fmt.Errorf("expected a stable tag vX.Y.Z, got %q", tag)
	}
	return tag[1:], nil
}

// Compare compares valid stable versions numerically without an integer size limit.
func Compare(a, b string) int {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for j := range x {
		if len(x[j]) < len(y[j]) {
			return -1
		}
		if len(x[j]) > len(y[j]) {
			return 1
		}
		if c := strings.Compare(x[j], y[j]); c != 0 {
			return c
		}
	}
	return 0
}

func AssetNames(version, goos, goarch string) (string, string, error) {
	if !Stable(version) {
		return "", "", fmt.Errorf("invalid stable version %q", version)
	}
	if (goos != "darwin" && goos != "linux") || (goarch != "amd64" && goarch != "arm64") {
		return "", "", fmt.Errorf("unsupported platform %s/%s; install a supported macOS or Linux binary", goos, goarch)
	}
	return fmt.Sprintf("wrk_%s_%s_%s.tar.gz", version, goos, goarch), "wrk_" + version + "_checksums.txt", nil
}
