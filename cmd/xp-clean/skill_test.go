package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	"github.com/kanzifucius/xp-clean/skills"
)

func TestSkillInstallHelp(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"skill", "install", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"--project", "--force", "--agent", "cursor", "SKILL.md"} {
		if !strings.Contains(s, want) {
			t.Fatalf("skill install help missing %q:\n%s", want, s)
		}
	}
}

func TestSkillInstallGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// UserHomeDir on some platforms also consults USERPROFILE / home dir cache;
	// set both common vars and rely on HOME for Unix.
	t.Setenv("USERPROFILE", home)

	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"skill", "install"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v\nout=%s", err, out.String())
	}

	want := filepath.Join(home, ".cursor", "skills", "xp-clean", "SKILL.md")
	got := strings.TrimSpace(out.String())
	if got != want {
		t.Fatalf("printed path: got %q want %q", got, want)
	}
	assertSkillFile(t, want)
}

func TestSkillInstallProject(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"skill", "install", "--project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install --project: %v\nout=%s", err, out.String())
	}

	want := filepath.Join(root, ".cursor", "skills", "xp-clean", "SKILL.md")
	got := strings.TrimSpace(out.String())
	if got != want {
		t.Fatalf("printed path: got %q want %q", got, want)
	}
	assertSkillFile(t, want)
}

func TestSkillInstallProjectFallsBackToCwd(t *testing.T) {
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"skill", "install", "--project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install --project without .git: %v", err)
	}
	want := filepath.Join(root, ".cursor", "skills", "xp-clean", "SKILL.md")
	if strings.TrimSpace(out.String()) != want {
		t.Fatalf("got %q want %q", out.String(), want)
	}
	assertSkillFile(t, want)
}

func TestSkillInstallRefusesOverwriteWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dest := filepath.Join(home, ".cursor", "skills", "xp-clean", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCommand()
	cmd.SetArgs([]string{"skill", "install"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when skill exists")
	}
	if exit.Status(err) != exit.Usage {
		t.Fatalf("exit status: got %d want %d (%v)", exit.Status(err), exit.Usage, err)
	}
	body, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != "old\n" {
		t.Fatalf("file should be unchanged, got %q", body)
	}
}

func TestSkillInstallForceOverwrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dest := filepath.Join(home, ".cursor", "skills", "xp-clean", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"skill", "install", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	assertSkillFile(t, dest)
}

func TestSkillInstallRejectsUnknownAgent(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"skill", "install", "--agent", "claude"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected usage error")
	}
	if exit.Status(err) != exit.Usage {
		t.Fatalf("exit status: got %d want %d (%v)", exit.Status(err), exit.Usage, err)
	}
	if !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("error should mention cursor: %v", err)
	}
}

func TestEmbeddedSkillTeachesGVKGlob(t *testing.T) {
	s := string(skills.SkillMarkdown)
	for _, want := range []string{
		"--gvk-glob",
		"--apply",
		"xp-clean.io/eligible",
		"dry-run",
		"roots",
		"instances-only",
		"foreground",
		"--wait",
		"--config",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("embedded SKILL.md missing %q", want)
		}
	}
}

func assertSkillFile(t *testing.T, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, skills.SkillMarkdown) {
		t.Fatalf("installed content mismatch (%d bytes vs embed %d)", len(got), len(skills.SkillMarkdown))
	}
	if !strings.Contains(string(got), "--gvk-glob") {
		t.Fatal("installed skill missing --gvk-glob")
	}
}
