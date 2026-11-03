package log

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestLevelFromVerbosity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    int
		want string
	}{
		{-1, "error"},
		{0, "error"},
		{1, "info"},
		{2, "debug"},
		{9, "debug"},
	}
	for _, tc := range cases {
		if got := LevelFromVerbosity(tc.v); got != tc.want {
			t.Fatalf("LevelFromVerbosity(%d)=%q want %q", tc.v, got, tc.want)
		}
	}
}

func TestSetupDefaultInfoWhenLevelEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger, _, err := Setup(Options{Out: &buf})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hello", "k", "v")
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("expected info log, got %q", buf.String())
	}
}

func TestSetupErrorLevelHidesInfo(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger, z, err := Setup(Options{Level: "error", Out: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if z.Core().Enabled(zapcore.InfoLevel) {
		t.Fatal("info must be disabled at error level")
	}
	logger.Info("should-not-appear")
	logger.Error(nil, "boom")
	out := buf.String()
	if strings.Contains(out, "should-not-appear") {
		t.Fatalf("info leaked at error level: %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Fatalf("expected error log, got %q", out)
	}
}
