package match

// LabelsMatch returns true when all required labels are present with the
// expected values. An empty require map means no label constraint.
func LabelsMatch(labels, require map[string]string) bool {
	if len(require) == 0 {
		return true
	}
	if labels == nil {
		return false
	}
	for k, v := range require {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// IsKeep reports whether the object carries xp-clean.io/keep=true.
func IsKeep(labels map[string]string) bool {
	return labels != nil && labels[LabelKeep] == "true"
}

// IsPaused reports whether Crossplane reconciliation is paused.
func IsPaused(labels, annotations map[string]string) bool {
	if labels != nil && labels[AnnotPaused] == "true" {
		return true
	}
	if annotations != nil && annotations[AnnotPaused] == "true" {
		return true
	}
	return false
}
