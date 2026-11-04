// Package skills embeds the Cursor skill markdown shipped with the xp-clean binary.
package skills

import _ "embed"

// SkillMarkdown is the Cursor SKILL.md for xp-clean (skills/xp-clean/SKILL.md).
//
//go:embed xp-clean/SKILL.md
var SkillMarkdown []byte
