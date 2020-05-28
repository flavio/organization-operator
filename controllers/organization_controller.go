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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8sv1alpha1 "github.com/flavio/organization-operator/api/v1alpha1"
	"github.com/flavio/organization-operator/pkg/common"
)

// OrganizationReconciler reconciles a Organization object
type OrganizationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8s.suse.com,resources=organizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.suse.com,resources=organizations/status,verbs=get;update;patch

func (r *OrganizationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// your logic here
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Organization")

	// Fetch the Organization instance
	instance := &k8sv1alpha1.Organization{}
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

	scopeNamespace := namespaceForOrganizationSpaceObjects(instance)
	if err := common.ReconcileNamespace(r, scopeNamespace, instance, r.Scheme, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Define a new RBAC Role that allows to read Scope objects inside of the namespace
	roleScopeReader := newRoleScopeReader(scopeNamespace)
	if err = r.reconcileRBACRole(roleScopeReader, instance, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Create a RoleBinding: all the viewers and editors of an Organization
	// can view the Scope objects related with the Organization
	roleBinding := common.NewRoleBinding(
		roleScopeReader.Name,
		roleScopeReader.Namespace,
		[]string{},
		append(instance.Spec.EditorGroups, instance.Spec.ViewerGroups...),
		rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleScopeReader.Name,
		},
	)
	if err = common.ReconcileRBACRoleBinding(r, roleBinding, instance, r.Scheme, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Define a new RBAC Role that allows to admin Scope objects inside of the namespace
	roleScopeAdmin := newRoleScopeAdmin(scopeNamespace)
	if err = r.reconcileRBACRole(roleScopeAdmin, instance, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}
	// Create a RoleBinding: only the admins of an Organization
	// can alter the Scope objects related with the Organization
	roleBinding = common.NewRoleBinding(
		roleScopeAdmin.Name,
		roleScopeAdmin.Namespace,
		[]string{},
		append(instance.Spec.EditorGroups, instance.Spec.ViewerGroups...),
		rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleScopeAdmin.Name,
		},
	)
	if err = common.ReconcileRBACRoleBinding(r, roleBinding, instance, r.Scheme, reqLogger, ctx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.Organization{}).
		Owns(&corev1.Namespace{}).
		Owns(&rbac.Role{}).
		Owns(&rbac.RoleBinding{}).
		Complete(r)
}

func namespaceForOrganizationSpaceObjects(cr *k8sv1alpha1.Organization) *corev1.Namespace {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   common.ComputeSpacesNamespaceFromOrganizationName(cr.Name),
			Labels: labels,
		},
	}
}

func newRoleScopeReader(namespace *corev1.Namespace) *rbac.Role {
	return &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scope-reader",
			Namespace: namespace.Name,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"k8s.suse.com/v1alpha1"},
				Resources: []string{"scopes"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func newRoleScopeAdmin(namespace *corev1.Namespace) *rbac.Role {
	return &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scope-admin",
			Namespace: namespace.Name,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"k8s.suse.com/v1alpha1"},
				Resources: []string{"scopes"},
				Verbs: []string{
					"get", "list", "watch",
					"create", "update", "patch", "delete"},
			},
		},
	}
}

func (r *OrganizationReconciler) reconcileRBACRole(
	role *rbac.Role,
	instance *k8sv1alpha1.Organization,
	reqLogger logr.Logger,
	ctx context.Context) error {
	// Set Organization instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, role, r.Scheme); err != nil {
		return err
	}

	// Check if this RBAC Role already exists
	found := &rbac.Role{}
	reqLogger.Info(
		"Looking for RBAC Role",
		"Namespace", role.Namespace,
		"Name", role.Name)

	err := r.Get(
		ctx,
		client.ObjectKey{
			Namespace: role.Namespace,
			Name:      role.Name,
		},
		found)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info(
			"Creating a new RBAC Role",
			"Role.Namespace", role.Namespace,
			"Role.Name", role.Name)
		err = r.Create(ctx, role)
		if err != nil {
			return err
		}

		reqLogger.Info(
			"Created RBAC Role",
			"Role.Namespace", role.Namespace,
			"Role.Name", role.Name)
		return nil
	} else if err != nil {
		return err
	}

	reqLogger.Info(
		"Found Role",
		"Namespace", found.Namespace,
		"Name", found.Name)

	if !common.ArePolicyRulesEqual(found.Rules, role.Rules) {
		found.Rules = role.Rules

		reqLogger.Info("Updating RBAC Role to have the same policy rules",
			"Namespace", found.Namespace,
			"Name", found.Name)
		return r.Client.Update(ctx, found)
	}

	return nil
}
