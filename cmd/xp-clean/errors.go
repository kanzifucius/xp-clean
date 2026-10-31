package main

import (
	"fmt"
	"strings"

	"github.com/kanzifucius/xp-clean/internal/cli/exit"
)

func classifyConfigErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid --scope") ||
		strings.Contains(msg, "invalid deletion.scope") ||
		strings.Contains(msg, "XP_CLEAN_DRY_RUN") ||
		strings.Contains(msg, "XP_CLEAN_SCOPE") ||
		strings.Contains(msg, "selectors must include") ||
		strings.Contains(msg, "policy source required") ||
		strings.Contains(msg, "no policy ConfigMaps found") {
		return exit.UsageErr(fmt.Errorf("config: %w", err))
	}
	return exit.RuntimeErr(fmt.Errorf("config: %w", err))
}
