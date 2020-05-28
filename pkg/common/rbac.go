package common

import (
	"bytes"
	"context"
	"encoding/json"

	logr "github.com/go-logr/logr"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func NewRoleBinding(name, namespace string, users, groups []string, roleRef rbac.RoleRef) *rbac.RoleBinding {
	subjects := []rbac.Subject{}
	for _, groupName := range groups {
		subject := rbac.Subject{
			Kind:     rbac.GroupKind,
			Name:     groupName,
			APIGroup: "rbac.authorization.k8s.io",
		}
		subjects = append(subjects, subject)
	}
	for _, user := range users {
		subject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     user,
			APIGroup: "rbac.authorization.k8s.io",
		}
		subjects = append(subjects, subject)
	}

	return &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: subjects,
		RoleRef:  roleRef,
	}
}

func ReconcileRBACRoleBinding(
	client client.Client,
	roleBinding *rbac.RoleBinding,
	owner metav1.Object,
	scheme *runtime.Scheme,
	reqLogger logr.Logger,
	ctx context.Context) error {
	// Set Organization instance as the owner and controller
	if owner != nil && scheme != nil {
		if err := controllerutil.SetControllerReference(owner, roleBinding, scheme); err != nil {
			return err
		}
	}

	// Check if this RBAC RoleBinding already exists
	found := &rbac.RoleBinding{}
	err := client.Get(
		ctx,
		types.NamespacedName{
			Name:      roleBinding.Name,
			Namespace: roleBinding.Namespace,
		},
		found)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info(
				"Creating a new RBAC RoleBinding",
				"Namespace", roleBinding.Namespace,
				"Name", roleBinding.Name,
				"Subjects", roleBinding.Subjects,
				"RoleRef", roleBinding.RoleRef)
			return client.Create(ctx, roleBinding)
		}
		return err
	}

	if !areRoleBindingsEqual(found, roleBinding) {
		reqLogger.Info(
			"Updating RBAC RoleBinding to have right set of Subjects and RoleRefs",
			"Name", found.Name,
			"Namespace", found.Namespace)
		found.Subjects = roleBinding.Subjects
		found.RoleRef = roleBinding.RoleRef
		return client.Update(ctx, found)
	}

	return nil
}

func ArePolicyRulesEqual(a, b []rbac.PolicyRule) bool {
	mA, err := json.Marshal(a)
	if err != nil {
		return false
	}

	mB, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return bytes.Equal(mA, mB)
}

func areRoleBindingsEqual(a, b *rbac.RoleBinding) bool {
	dataA, err := json.Marshal(a.Subjects)
	if err != nil {
		return false
	}

	dataB, err := json.Marshal(b.Subjects)
	if err != nil {
		return false
	}

	if !bytes.Equal(dataA, dataB) {
		return false
	}

	dataA, err = json.Marshal(a.RoleRef)
	if err != nil {
		return false
	}

	dataB, err = json.Marshal(b.RoleRef)
	if err != nil {
		return false
	}

	if !bytes.Equal(dataA, dataB) {
		return false
	}

	return true
}
