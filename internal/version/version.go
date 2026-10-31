package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

// Build-time variables; override via -ldflags "-X …".
var (
	Version   = "0.1.0-dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

// Info is the structured version payload.
type Info struct {
	Version   string `json:"version" yaml:"version"`
	GitCommit string `json:"gitCommit" yaml:"gitCommit"`
	BuildDate string `json:"buildDate" yaml:"buildDate"`
	GoVersion string `json:"goVersion" yaml:"goVersion"`
	Platform  string `json:"platform" yaml:"platform"`
}

// Get returns build metadata.
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// Format renders version for -o short|json|yaml (yaml is simple key lines for MVP).
func Format(output string) (string, error) {
	info := Get()
	switch strings.ToLower(output) {
	case "", "short":
		return info.Version, nil
	case "json":
		b, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	case "yaml":
		return fmt.Sprintf(
			"version: %s\ngitCommit: %s\nbuildDate: %s\ngoVersion: %s\nplatform: %s\n",
			info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.Platform,
		), nil
	default:
		return "", fmt.Errorf("unknown version output %q (want short|json|yaml)", output)
	}
}
