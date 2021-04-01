package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/riita10069/rolling-update-status/controllers/pkg"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ReplicaSetReconciler reconciles a Deployment object
type ReplicaSetReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	StoreRepo pkg.StoreRepository
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch

func (r *ReplicaSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	var rs appsv1.ReplicaSet
	err := r.Get(ctx, req.NamespacedName, &rs)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	gs := pkg.NewGithubStatusWithRs(rs)

	gs.CreatePending()
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func (r *ReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ReplicaSet{}).
		WithEventFilter(pred).
		Complete(r)
}
