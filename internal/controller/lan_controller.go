/*
Copyright 2025.

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

package controller

import (
	"context"

	"github.com/hujun-open/k8slan/api/v1beta1"
	ncv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// LANReconciler reconciles a LAN object
type LANReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=lan.k8slan.io,resources=lans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lan.k8slan.io,resources=lans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lan.k8slan.io,resources=lans/finalizers,verbs=update

//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete;deletecollection

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LAN object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *LANReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("reconcile started", "request", req)
	lan := new(v1beta1.LAN)
	if err := r.Get(ctx, req.NamespacedName, lan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	nads := lan.Spec.GetNADs(req.Namespace)
	existingNads := new(ncv1.NetworkAttachmentDefinitionList)
	if err := r.List(ctx, existingNads); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	for _, nad := range nads {
		found := false
		for _, enad := range existingNads.Items {
			if nad.Name == enad.Name {
				found = true
				break
			}
		}
		if !found {
			//create it
			//mark owner
			err := ctrl.SetControllerReference(lan, nad, r.Scheme)
			if err != nil {
				logger.Error(err, "failed to set owner reference for nad", "nad", nad.Name)
			} else {
				err = r.Client.Create(ctx, nad)
				if err != nil {
					logger.Error(err, "failed to create nad", "nad", nad.Name)
				}
			}

		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LANReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := registrResource[ncv1.NetworkAttachmentDefinition](context.Background(), mgr); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&v1beta1.LAN{}).
		Owns(&ncv1.NetworkAttachmentDefinition{}).
		Named("lan").
		Complete(r)
}

// see https://stackoverflow.com/questions/69573113/how-can-i-instantiate-a-non-nil-pointer-of-type-argument-with-generic-go
type myObj[B any] interface {
	client.Object
	*B
}

func extractKey[T client.Object](rawObj client.Object) []string {
	job := rawObj.(T)
	owner := metav1.GetControllerOf(job)
	if owner == nil {
		return nil
	}

	if owner.APIVersion != v1beta1.GroupVersion.String() || owner.Kind != "Lab" {
		return nil
	}

	// ...and if so, return it
	return []string{owner.Name}
}

func registrResource[T any, PT myObj[T]](ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx,
		PT(new(T)),
		".metadata.controller",
		extractKey[PT])
}
