// this is a daemonset
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/hujun-open/k8slan/api/v1beta1"
	k8slan "github.com/hujun-open/k8slan/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ============================================================================
// Controller Implementation
// ============================================================================

type LANReconciler struct {
	client.Client
	hostName string
}

// +kubebuilder:rbac:groups=lan.k8slan.io,resources=lans,verbs=get;list;watch;update

func makeFinalizerPatch(in v1beta1.LAN, fin string) client.Patch {
	p := &v1beta1.LAN{}
	p.APIVersion = in.APIVersion
	p.Kind = in.Kind
	p.Name = in.Name
	p.Namespace = in.Namespace
	p.Finalizers = append(p.Finalizers, fin)
	patchBytes, _ := json.Marshal(p)
	return client.RawPatch(types.MergePatchType, patchBytes)

}

func (r *LANReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.Log.WithValues("lan", req.NamespacedName)

	lan := &k8slan.LAN{}
	if err := r.Get(ctx, req.NamespacedName, lan); err != nil {
		log.Error(err, "unable to fetch LAN")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	myFinalizerName := fmt.Sprintf("finalizer.k8slan.io/%v", r.hostName)
	fieldOwner := fmt.Sprintf("fieldowner.k8slan.io/%v", r.hostName)
	if lan.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then let's add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(lan, myFinalizerName) {
			patch := makeFinalizerPatch(*lan, myFinalizerName)
			if err := r.Patch(ctx, lan, patch, &client.PatchOptions{FieldManager: fieldOwner}); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add finalizer,%w", err)
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(lan, myFinalizerName) {
			// our finalizer is present, so let's handle any external dependency
			r.remove(lan)
			// remove our finalizer from the list and update it.
			patch := client.MergeFrom(lan.DeepCopy())
			controllerutil.RemoveFinalizer(lan, myFinalizerName)
			if err := r.Patch(ctx, lan, patch); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}
	err := r.ensure(lan)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("lan created", "name", lan.Name)
	return reconcile.Result{}, nil
}

func (r *LANReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8slan.LAN{}).
		Complete(r)
}

// ============================================================================
// Main Function
// ============================================================================

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	hostName, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	// Create scheme and register custom resource types
	scheme := runtime.NewScheme()
	k8slan.AddToScheme(scheme)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to start manager: %v\n", err)
		os.Exit(1)
	}

	if err = (&LANReconciler{
		Client: mgr.GetClient(), hostName: hostName,
	}).SetupWithManager(mgr); err != nil {
		fmt.Fprintf(os.Stderr, "unable to create controller: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "unable to start manager: %v\n", err)
		os.Exit(1)
	}
}
