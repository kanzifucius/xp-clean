package controller

import (
	"context"
	"log/slog"

	"github.com/kanzifucius/xp-clean/internal/conf"
)

// CleanupPolicyReconciler is an MVP stub for the future CleanupPolicy CRD watch.
// For now it loads a file/ConfigMap policy and hands it to the shared Runner.
type CleanupPolicyReconciler struct {
	Policy conf.Resolved
	Run    RunnerFunc
	Log    *slog.Logger
}

// ReconcileOnce executes a single cleanup pass (used by `xp-clean manager` stub
// and by tests). Full controller-runtime wiring is deferred past MVP scaffold.
func (r *CleanupPolicyReconciler) ReconcileOnce(ctx context.Context) error {
	if r.Log == nil {
		r.Log = slog.Default()
	}
	if r.Run == nil {
		r.Log.Info("reconcile stub: no runner configured", "policy", r.Policy.Name)
		return nil
	}
	r.Log.Info("reconcile once", "policy", r.Policy.Name, "dryRun", r.Policy.DryRun, "scope", r.Policy.Deletion.Scope)
	return r.Run(ctx, r.Policy)
}
