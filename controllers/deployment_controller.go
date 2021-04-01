/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/riita10069/rolling-update-status/controllers/pkg"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	StoreRepo pkg.StoreRepository
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch

func (r *DeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	// _ = r.Log.WithValues("deployment", req.NamespacedName)

	var dply appsv1.Deployment
	err := r.Get(ctx, req.NamespacedName, &dply)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	r.StoreRepo = pkg.NewStore(dply)

	stmt, status, err := pkg.RolloutStatus(dply)
	fmt.Println("rollout status", stmt, status)
	if stmt == pkg.RevisionNotFound {
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}
	if stmt == pkg.TimedOutReason {
		var ok bool
		if ok, err = pkg.ValidateNewReplicaSet(dply, r.Client); err != nil {
			if err == pkg.NewReplicaSetNotFound {
				return ctrl.Result{Requeue: true}, err
			} else {
				return ctrl.Result{}, err
			}
		}
		if !ok {
			return ctrl.Result{}, nil
		} else {
			if err := r.StoreRepo.Create("ok", "error: timed out waiting for any update progress to be made"); err != nil {
				return ctrl.Result{}, err
			}
			return reconcile.Result{}, nil
		}
	}
	if !status {
		return ctrl.Result{}, nil
	} else {
		var ok bool
		if ok, err = pkg.ValidateNewReplicaSet(dply, r.Client); err != nil {
			if err == pkg.NewReplicaSetNotFound {
				return ctrl.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, err
		} else {
			fmt.Println("is just ok?", ok)
			if !ok {
				return reconcile.Result{}, nil
			} else {
				if err = r.StoreRepo.Create("success", "the cluster ok the Rolling Update."); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			}
		}
	}
}

func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		WithEventFilter(pred).
		Complete(r)
}
