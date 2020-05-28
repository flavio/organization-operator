package common

import (
	"bytes"
	"context"
	"encoding/json"

	logr "github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func ReconcileNamespace(
	client client.Client,
	namespace *corev1.Namespace,
	owner metav1.Object,
	scheme *runtime.Scheme,
	reqLogger logr.Logger,
	ctx context.Context) error {
	// Set Organization instance as the owner and controller
	if owner != nil && scheme != nil {
		if err := controllerutil.SetControllerReference(owner, namespace, scheme); err != nil {
			return err
		}
	}

	// Check if this Namespace already exists
	found := &corev1.Namespace{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: namespace.Name}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info(
				"Creating a new Namespace",
				"Namespace", namespace.Name,
				"Labels", namespace.Labels)
			return client.Create(ctx, namespace)
		}
		return err
	}

	if !namespacesHaveSameLabels(found, namespace) {
		found.SetLabels(namespace.GetLabels())
		reqLogger.Info("Updating namespace to have the right set of labels",
			"Namespace", found.Name)
		return client.Update(ctx, found)
	}

	return nil
}

func namespacesHaveSameLabels(a, b *corev1.Namespace) bool {
	mA, err := json.Marshal(a.GetLabels())
	if err != nil {
		return false
	}

	mB, err := json.Marshal(b.GetLabels())
	if err != nil {
		return false
	}

	return bytes.Equal(mA, mB)
}
