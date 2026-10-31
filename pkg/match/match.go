package match

import "fmt"

// Target identifies which string fields are evaluated for globs.
type Target string

const (
	TargetXRD Target = "xrd"
	TargetXR  Target = "xr"
	TargetGVK Target = "gvk"
)

// Input is everything needed to decide eligibility for one object.
type Input struct {
	Meta    ObjectMeta
	Group   string
	Version string
	Kind    string
	Target  Target
}

// Result is the eligibility decision.
type Result struct {
	Eligible bool
	Keep     bool
	Paused   bool
	Denied   bool
	Reason   string
}

// Evaluate applies label and/or glob selectors (AND of non-empty sides),
// namespace deny list, and keep/paused flags.
func Evaluate(in Input, sel Selectors) (Result, error) {
	if IsKeep(in.Meta.Labels) {
		return Result{Eligible: false, Keep: true, Reason: "keep"}, nil
	}

	denied, err := NamespaceDenied(in.Meta.Namespace, sel.NamespaceDeny)
	if err != nil {
		return Result{}, err
	}
	if denied {
		return Result{Eligible: false, Denied: true, Reason: "namespace_deny"}, nil
	}

	hasLabelConstraint := len(sel.RequireLabels) > 0
	hasGlobConstraint := hasAnyGlobs(sel.Globs)
	if !hasLabelConstraint && !hasGlobConstraint {
		return Result{Eligible: false, Reason: "empty_selectors"}, nil
	}

	labelsOK := LabelsMatch(in.Meta.Labels, sel.RequireLabels)

	globsOK := true
	if hasGlobConstraint {
		globsOK, err = matchGlobsForTarget(in, sel.Globs)
		if err != nil {
			return Result{}, err
		}
	}

	if hasLabelConstraint && !labelsOK {
		return Result{Eligible: false, Reason: "labels_mismatch"}, nil
	}
	if hasGlobConstraint && !globsOK {
		return Result{Eligible: false, Reason: "globs_mismatch"}, nil
	}

	paused := IsPaused(in.Meta.Labels, in.Meta.Annotations)
	return Result{
		Eligible: true,
		Paused:   paused,
		Reason:   "matched",
	}, nil
}

func hasAnyGlobs(g GlobPatterns) bool {
	return len(g.Namespaces) > 0 || len(g.XRDNames) > 0 || len(g.XRNames) > 0 || len(g.GVKs) > 0
}

func matchGlobsForTarget(in Input, g GlobPatterns) (bool, error) {
	// Namespace glob: if set, object namespace must match (cluster-scoped XRDs skip).
	if len(g.Namespaces) > 0 && in.Meta.Namespace != "" {
		ok, err := MatchAny(in.Meta.Namespace, g.Namespaces)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	switch in.Target {
	case TargetXRD:
		if len(g.XRDNames) > 0 {
			ok, err := MatchAny(in.Meta.Name, g.XRDNames)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
	case TargetXR:
		if len(g.XRNames) > 0 {
			ok, err := MatchAny(in.Meta.Name, g.XRNames)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
	case TargetGVK:
		// GVK-only checks fall through to GVK patterns below.
	default:
		return false, fmt.Errorf("unknown match target %q", in.Target)
	}

	if len(g.GVKs) > 0 {
		ok, err := MatchGVK(in.Group, in.Version, in.Kind, g.GVKs)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}
