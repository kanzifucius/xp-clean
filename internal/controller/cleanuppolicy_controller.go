package controller

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kanzifucius/xp-clean/internal/conf"
)

// CleanupPolicyReconciler is an MVP stub for the future CleanupPolicy CRD watch.
// Today it holds resolved ConfigMap/file policies and hands them to the shared Runner.
type CleanupPolicyReconciler struct {
	Policies []conf.Resolved
	Run      RunnerFunc
	Log      logr.Logger
}

// ReconcileOnce executes a single cleanup pass per policy (used by `xp-clean manager`
// stub and by tests). Full controller-runtime wiring is deferred past MVP.
func (r *CleanupPolicyReconciler) ReconcileOnce(ctx context.Context) error {
	if r.Log.GetSink() == nil {
		r.Log = logr.Discard()
	}
	if r.Run == nil {
		r.Log.Info("reconcile stub: no runner configured", "policies", len(r.Policies))
		return nil
	}
	for _, p := range r.Policies {
		r.Log.Info("reconcile once", "policy", p.Name, "dryRun", p.DryRun, "scope", p.Deletion.Scope)
		if err := r.Run(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
