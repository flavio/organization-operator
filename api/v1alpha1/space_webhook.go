/*
Copyright 2020 SUSE.

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

package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/flavio/organization-operator/pkg/common"
)

// log is for logging in this package.
var spacelog = logf.Log.WithName("space-resource")

func (r *Space) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-k8s-suse-com-v1alpha1-space,mutating=true,failurePolicy=fail,groups=k8s.suse.com,resources=spaces,verbs=create;update,versions=v1alpha1,name=mspace.kb.io

var _ webhook.Defaulter = &Space{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
// Mutation webhook for Space objects. It ensures a proper finalizer is set.
func (r *Space) Default() {
	spacelog.Info("Setting default values for Space object",
		"Namespace", r.Namespace,
		"Name", r.Name)
	spaceFinalizerFound := false

	finalizers := r.ObjectMeta.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == common.SpaceFinalizer {
			spaceFinalizerFound = true
		}
	}

	if !spaceFinalizerFound {
		finalizers = append(finalizers, common.SpaceFinalizer)
		r.SetFinalizers(finalizers)
	}
}
