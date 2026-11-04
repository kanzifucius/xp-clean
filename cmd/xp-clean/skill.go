package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	"github.com/kanzifucius/xp-clean/skills"
	"github.com/spf13/cobra"
)

const (
	skillAgentCursor = "cursor"
	skillRelDir      = "skills/xp-clean"
	skillFileName    = "SKILL.md"
)

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the Cursor skill for xp-clean",
		Long: `Install the embedded Cursor skill that teaches safe xp-clean usage
(dry-run first, confirm before --apply, skinny eligibility, --gvk-glob).`,
	}
	cmd.AddCommand(newSkillInstallCommand())
	return cmd
}

func newSkillInstallCommand() *cobra.Command {
	var (
		project bool
		force   bool
		agent   string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the embedded Cursor skill (global by default)",
		Long: `Write the embedded skills/xp-clean/SKILL.md to disk.

Default (global): ~/.cursor/skills/xp-clean/SKILL.md
--project:        .cursor/skills/xp-clean/SKILL.md under the git root (or cwd)

v1 supports -a/--agent=cursor only. Use --force to overwrite an existing file.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			agent = strings.TrimSpace(strings.ToLower(agent))
			if agent == "" {
				agent = skillAgentCursor
			}
			if agent != skillAgentCursor {
				return exit.UsageErr(fmt.Errorf("unsupported agent %q (v1 supports %q only)", agent, skillAgentCursor))
			}

			dest, err := skillInstallPath(project)
			if err != nil {
				return exit.RuntimeErr(err)
			}
			if err := writeSkillFile(dest, skills.SkillMarkdown, force); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), dest)
			return err
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "install under .cursor/skills/xp-clean/ in the current git root (or cwd)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing skill file")
	cmd.Flags().StringVarP(&agent, "agent", "a", skillAgentCursor, "agent to install for (v1: cursor only)")
	return cmd
}

func skillInstallPath(project bool) (string, error) {
	if project {
		root, err := findProjectRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(root, ".cursor", skillRelDir, skillFileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cursor", skillRelDir, skillFileName), nil
}

// findProjectRoot walks up from cwd looking for .git; falls back to cwd.
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd, nil
		}
		dir = parent
	}
}

func writeSkillFile(dest string, content []byte, force bool) error {
	if len(content) == 0 {
		return exit.RuntimeErr(fmt.Errorf("embedded skill is empty"))
	}
	if !force {
		if _, err := os.Stat(dest); err == nil {
			return exit.UsageErr(fmt.Errorf("skill already exists at %s (use --force to overwrite)", dest))
		} else if !os.IsNotExist(err) {
			return exit.RuntimeErr(fmt.Errorf("stat %s: %w", dest, err))
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return exit.RuntimeErr(fmt.Errorf("create skill directory: %w", err))
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), "SKILL.md.*.tmp")
	if err != nil {
		return exit.RuntimeErr(fmt.Errorf("create temp skill file: %w", err))
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return exit.RuntimeErr(fmt.Errorf("write skill: %w", err))
	}
	if err := tmp.Close(); err != nil {
		return exit.RuntimeErr(fmt.Errorf("close skill temp: %w", err))
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return exit.RuntimeErr(fmt.Errorf("install skill to %s: %w", dest, err))
	}
	return nil
}
