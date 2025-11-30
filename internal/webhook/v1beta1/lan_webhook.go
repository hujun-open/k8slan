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

package v1beta1

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/hujun-open/k8slan/api/v1beta1"
	lanv1beta1 "github.com/hujun-open/k8slan/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var lanlog = logf.Log.WithName("lan-resource")

// SetupLANWebhookWithManager registers the webhook for LAN in the manager.
func SetupLANWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&lanv1beta1.LAN{}).
		WithValidator(&LANCustomValidator{}).
		WithDefaulter(&LANCustomDefaulter{
			vxport: v1beta1.DefaultVxPort,
			vxgrp:  v1beta1.DefaultVxGrp,
		}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-lan-k8slan-io-v1beta1-lan,mutating=true,failurePolicy=fail,sideEffects=None,groups=lan.k8slan.io,resources=lans,verbs=create;update,versions=v1beta1,name=mlan-v1beta1.kb.io,admissionReviewVersions=v1

// LANCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind LAN when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type LANCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
	vxport int32
	vxgrp  string
}

// SetDefaultGeneric return inval if it is not nil, otherwise return defVal
func SetDefaultGeneric[T any](inval *T, defVal T) *T {
	if inval != nil {
		return inval
	}
	r := new(T)
	*r = defVal
	return r
}

var _ webhook.CustomDefaulter = &LANCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind LAN.
func (d *LANCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	lan, ok := obj.(*lanv1beta1.LAN)

	if !ok {
		return fmt.Errorf("expected an LAN object but got %T", obj)
	}
	lanlog.Info("Defaulting for LAN", "name", lan.GetName())
	lan.Spec.VxPort = SetDefaultGeneric(lan.Spec.VxPort, d.vxport)
	lan.Spec.VxLANGrp = SetDefaultGeneric(lan.Spec.VxLANGrp, d.vxgrp)

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-lan-k8slan-io-v1beta1-lan,mutating=false,failurePolicy=fail,sideEffects=None,groups=lan.k8slan.io,resources=lans,verbs=create;update,versions=v1beta1,name=vlan-v1beta1.kb.io,admissionReviewVersions=v1

// LANCustomValidator struct is responsible for validating the LAN resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type LANCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &LANCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type LAN.
func (v *LANCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	lan, ok := obj.(*lanv1beta1.LAN)
	if !ok {
		return nil, fmt.Errorf("expected a LAN object but got %T", obj)
	}
	lanlog.Info("Validation for LAN upon creation", "name", lan.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, lan.Spec.Validate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type LAN.
func (v *LANCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	lan, ok := newObj.(*lanv1beta1.LAN)
	if !ok {
		return nil, fmt.Errorf("expected a LAN object for the newObj but got %T", newObj)
	}
	old, ok := oldObj.(*lanv1beta1.LAN)
	if !ok {
		return nil, fmt.Errorf("expected a LAN object for the oldObj but got %T", newObj)
	}

	lanlog.Info("Validation for LAN upon update", "name", lan.GetName())

	if !reflect.DeepEqual(lan.Spec, old.Spec) {
		return nil, field.Forbidden(
			field.NewPath("spec"),
			"updates to the spec are not allowed; delete and recreate the resource instead",
		)
	}

	return nil, lan.Spec.Validate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type LAN.
func (v *LANCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	lan, ok := obj.(*lanv1beta1.LAN)
	if !ok {
		return nil, fmt.Errorf("expected a LAN object but got %T", obj)
	}
	lanlog.Info("Validation for LAN upon deletion", "name", lan.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
