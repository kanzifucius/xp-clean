package options

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestQuietCLILoggingDefault(t *testing.T) {
	t.Parallel()
	g := NewGlobal()
	if g.Log.Level != "error" || g.Log.Devel {
		t.Fatalf("quiet defaults: level=%q devel=%v", g.Log.Level, g.Log.Devel)
	}

	cmd := &cobra.Command{Use: "plan"}
	root := &cobra.Command{Use: "xp-clean"}
	root.AddCommand(cmd)
	g.BindRootFlags(root.PersistentFlags())
	root.SetArgs([]string{"plan"})
	if err := root.ParseFlags([]string{"plan"}); err != nil {
		t.Fatal(err)
	}
	if err := g.CompleteLogging(cmd); err != nil {
		t.Fatal(err)
	}
	if g.Logger.GetSink().Enabled(0) {
		t.Fatal("info logging must be disabled by default on plan")
	}
}

func TestVerbosityFlagEnablesInfo(t *testing.T) {
	t.Parallel()
	g := NewGlobal()
	cmd := &cobra.Command{Use: "plan"}
	root := &cobra.Command{Use: "xp-clean"}
	root.AddCommand(cmd)
	g.BindRootFlags(root.PersistentFlags())
	if err := root.PersistentFlags().Parse([]string{"-v=1"}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	g.Log.Out = &buf
	if err := g.CompleteLogging(cmd); err != nil {
		t.Fatal(err)
	}
	if !g.Logger.GetSink().Enabled(0) {
		t.Fatal("expected info enabled at -v=1")
	}
	g.Logger.Info("verbose-line")
	if !bytes.Contains(buf.Bytes(), []byte("verbose-line")) {
		t.Fatalf("expected verbose log, got %q", buf.String())
	}
}

func TestManagerDefaultsToInfo(t *testing.T) {
	t.Parallel()
	g := NewGlobal()
	cmd := &cobra.Command{Use: "manager"}
	root := &cobra.Command{Use: "xp-clean"}
	root.AddCommand(cmd)
	g.BindRootFlags(root.PersistentFlags())
	if err := root.PersistentFlags().Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	if err := g.CompleteLogging(cmd); err != nil {
		t.Fatal(err)
	}
	if g.Log.Level != "info" {
		t.Fatalf("manager default level=%q want info", g.Log.Level)
	}
	if !g.Logger.GetSink().Enabled(0) {
		t.Fatal("manager should enable info by default")
	}
}

func TestExplicitErrorKeepsManagerQuiet(t *testing.T) {
	t.Parallel()
	g := NewGlobal()
	cmd := &cobra.Command{Use: "manager"}
	root := &cobra.Command{Use: "xp-clean"}
	root.AddCommand(cmd)
	g.BindRootFlags(root.PersistentFlags())
	if err := root.PersistentFlags().Parse([]string{"--log-level=error"}); err != nil {
		t.Fatal(err)
	}
	if err := g.CompleteLogging(cmd); err != nil {
		t.Fatal(err)
	}
	if g.Logger.GetSink().Enabled(0) {
		t.Fatal("explicit --log-level=error must keep manager quiet")
	}
}
