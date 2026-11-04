package match

import (
	"fmt"
	"sort"
	"strings"
)

// MatchedReason builds a human/agent-readable eligibility cause for a successful
// selector match. Examples: label:xp-clean.io/eligible=true, labels, globs,
// labels+globs, label:xp-clean.io/eligible=true+globs.
func MatchedReason(sel Selectors) string {
	var parts []string
	if len(sel.RequireLabels) > 0 {
		parts = append(parts, formatLabelsReason(sel.RequireLabels))
	}
	if hasAnyGlobs(sel.Globs) {
		parts = append(parts, "globs")
	}
	if len(parts) == 0 {
		return "matched"
	}
	return strings.Join(parts, "+")
}

func formatLabelsReason(require map[string]string) string {
	if len(require) == 1 {
		for k, v := range require {
			return "label:" + k + "=" + v
		}
	}
	keys := make([]string, 0, len(require))
	for k := range require {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	chunks := make([]string, 0, len(keys))
	for _, k := range keys {
		chunks = append(chunks, fmt.Sprintf("%s=%s", k, require[k]))
	}
	return "labels:" + strings.Join(chunks, ",")
}

// TTLElapsedReason maps an expiry evaluation to a compact delete-reason fragment.
// Empty when TTL did not contribute to eligibility (no gate / no annotation).
func TTLElapsedReason(exp ExpiryResult) string {
	if !exp.Expired {
		return ""
	}
	switch exp.Reason {
	case "expires_at":
		return "expires-at"
	case "ttl":
		return "ttl_elapsed"
	case "default_ttl":
		return "default_ttl"
	case "no_ttl", "ttl_not_required":
		return ""
	default:
		return exp.Reason
	}
}

// JoinEligibility combines selector and TTL cause fragments with "+".
// Empty parts are omitted; when both are empty returns "".
func JoinEligibility(selectorReason, ttlReason string) string {
	selectorReason = strings.TrimSpace(selectorReason)
	ttlReason = strings.TrimSpace(ttlReason)
	switch {
	case selectorReason == "" && ttlReason == "":
		return ""
	case selectorReason == "":
		return ttlReason
	case ttlReason == "":
		return selectorReason
	default:
		return selectorReason + "+" + ttlReason
	}
}
