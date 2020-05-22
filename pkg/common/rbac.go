package common

import (
	"context"

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
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info(
			"Creating a new RBAC RoleBinding",
			"Namespace", roleBinding.Namespace,
			"Name", roleBinding.Name,
			"Subjects", roleBinding.Subjects,
			"RoleRef", roleBinding.RoleRef)
		err = client.Create(ctx, roleBinding)
		if err != nil {
			return err
		}

		reqLogger.Info(
			"Created RBAC RoleBinding",
			"Role.Namespace", roleBinding.Namespace,
			"Role.Name", roleBinding.Name)
	} else if err != nil {
		return err
	}

	return nil
}
