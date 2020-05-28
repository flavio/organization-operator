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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	k8sv1alpha1 "github.com/flavio/organization-operator/api/v1alpha1"
	"github.com/flavio/organization-operator/pkg/common"
)

const labelOrganization = "organization-operator.k8s.suse.com/organization"
const labelSpace = "organization-operator.k8s.suse.com/space"

// SpaceReconciler reconciles a Space object
type SpaceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8s.suse.com,resources=spaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.suse.com,resources=spaces/status,verbs=get;update;patch

func (r *SpaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("space", req.NamespacedName)

	reqLogger.Info("Reconciling Space")

	// Fetch the Space instance
	instance := &k8sv1alpha1.Space{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile req.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the req.
		return ctrl.Result{}, err
	}

	organizationName, err := common.ComputeOrganizationNameFromSpaceNamespace(req.Namespace)
	if err != nil {
		reqLogger.Error(err, "Cannot deduce organization name")
	}
	organization, err := r.organizationOwningSpace(organizationName, reqLogger, ctx)
	if err != nil {
		reqLogger.Info(
			"Cannot find organization owning space",
			"Space.Namespace", instance.Namespace,
			"Space.Name", instance.Name,
			"error", err)
		return ctrl.Result{}, err
	}
	reqLogger.Info("Organization found")

	beingDeleted := instance.GetDeletionTimestamp() != nil
	if beingDeleted {
		return r.handleFinalizer(instance, organization, reqLogger, ctx)
	}

	namespaceCR := namespaceAssociatedWithSpace(instance, organization)
	reqLogger.Info(
		"Reconciling Namespace associated with Space",
		"Namespace", namespaceCR.Name)
	err = common.ReconcileNamespace(
		r,
		namespaceCR,
		nil,
		nil,
		reqLogger,
		ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	roleBindingLabels := map[string]string{
		labelOrganization: organization.Name,
		labelSpace:        instance.Name,
	}

	// Create a RoleBinding for admin groups and users
	roleBinding := common.NewRoleBinding(
		"administrators",
		namespaceCR.Name,
		instance.Spec.Admins,
		append(organization.Spec.AdminGroups, instance.Spec.AdminGroups...),
		rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	)
	reqLogger.Info(
		"Reconciling RoleBinding",
		"Namespace", namespaceCR.Name,
		"RoleBinding", roleBinding.Name)
	roleBinding.ObjectMeta.SetLabels(roleBindingLabels)
	if err = common.ReconcileRBACRoleBinding(r, roleBinding, nil, nil, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Create a RoleBinding for editor groups and users
	roleBinding = common.NewRoleBinding(
		"editors",
		namespaceCR.Name,
		instance.Spec.Editors,
		append(organization.Spec.EditorGroups, instance.Spec.EditorGroups...),
		rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
	)
	roleBinding.ObjectMeta.SetLabels(roleBindingLabels)
	reqLogger.Info(
		"Reconciling RoleBinding",
		"Namespace", namespaceCR.Name,
		"RoleBinding", roleBinding.Name)
	if err = common.ReconcileRBACRoleBinding(r, roleBinding, nil, nil, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Create a RoleBinding for viewer groups and users
	roleBinding = common.NewRoleBinding(
		"viewers",
		namespaceCR.Name,
		instance.Spec.Viewers,
		append(organization.Spec.ViewerGroups, instance.Spec.ViewerGroups...),
		rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
	)
	roleBinding.ObjectMeta.SetLabels(roleBindingLabels)
	reqLogger.Info(
		"Reconciling RoleBinding",
		"Namespace", namespaceCR.Name,
		"RoleBinding", roleBinding.Name)
	if err = common.ReconcileRBACRoleBinding(r, roleBinding, nil, nil, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SpaceReconciler) organizationOwningSpace(orgName string, reqLogger logr.Logger, ctx context.Context) (*k8sv1alpha1.Organization, error) {
	reqLogger.Info(
		"Searching for organization owning space",
		"Organization.Name", orgName)
	organization := &k8sv1alpha1.Organization{}
	err := r.Get(
		ctx,
		client.ObjectKey{
			Namespace: "",
			Name:      orgName,
		},
		organization)

	return organization, err
}

func (r *SpaceReconciler) handleFinalizer(instance *k8sv1alpha1.Space, organization *k8sv1alpha1.Organization, reqLogger logr.Logger, ctx context.Context) (ctrl.Result, error) {
	finalizerFound := false
	newFinalizers := []string{}
	for _, finalizer := range instance.GetFinalizers() {
		if finalizer == common.SpaceFinalizer {
			finalizerFound = true
		} else {
			newFinalizers = append(newFinalizers, finalizer)
		}
	}

	if finalizerFound {
		// Perform finalization logic. If this fails, leave the finalizer
		// intact and requeue the reconcile request to attempt the clean
		// up again without allowing Kubernetes to actually delete
		// the resource.
		reqLogger.Info("Handling finalizer")

		namespace := namespaceAssociatedWithSpace(instance, organization)
		err := r.Delete(ctx, namespace)
		if err != nil {
			if !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			} else {
				reqLogger.Info("Cannot find Namespace associated with Space",
					"Namespace.Name", namespace.Name)
			}
		} else {
			reqLogger.Info("Deleted Namespace related with Space",
				"Namespace.Name", namespace.Name)
		}

		instance.SetFinalizers(newFinalizers)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *SpaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr)
	builder = builder.For(&k8sv1alpha1.Space{})

	// Watch for changes to the namespace related with the resource Space
	// Note well: we cannot leverage an ownership relation because Space
	// is a namespace-scoped entity while Namespace is a cluster-wide one
	builder = builder.Watches(
		&source.Kind{Type: &corev1.Namespace{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []ctrl.Request {
				labels := a.Meta.GetLabels()
				space, foundSpace := labels[labelSpace]
				org, foundOrg := labels[labelOrganization]

				requests := []ctrl.Request{}
				if foundSpace && foundOrg {
					requests = []ctrl.Request{
						{
							NamespacedName: client.ObjectKey{
								Name:      space,
								Namespace: common.ComputeSpacesNamespaceFromOrganizationName(org),
							},
						},
					}
				}
				return requests
			},
			),
		},
	)

	// Watch for changes to the RoleBinding objects created inside of the
	// Namespace created by the Space resource.
	// Note well: we cannot leverage an ownership relation because the Space
	// object and the RoleBinding ones are not under the same Namespace.
	// "Cross namespace ownership relations" are not supported.
	builder = builder.Watches(
		&source.Kind{Type: &rbac.RoleBinding{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []ctrl.Request {
				labels := a.Meta.GetLabels()
				space, foundSpace := labels[labelSpace]
				org, foundOrg := labels[labelOrganization]

				requests := []ctrl.Request{}
				if foundSpace && foundOrg {
					requests = []ctrl.Request{
						{
							NamespacedName: client.ObjectKey{
								Name:      space,
								Namespace: common.ComputeSpacesNamespaceFromOrganizationName(org),
							},
						},
					}
				}
				return requests
			},
			),
		},
	)

	return builder.Complete(r)
}

func namespaceAssociatedWithSpace(space *k8sv1alpha1.Space, organization *k8sv1alpha1.Organization) *corev1.Namespace {
	name := common.NameOfNamespaceCreateBySpace(organization.Name, space.Name)
	labels := organization.Spec.DefaultNamespaceLabels
	labels[labelOrganization] = organization.Name
	labels[labelSpace] = space.Name

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
