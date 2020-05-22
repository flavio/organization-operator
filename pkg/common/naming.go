package common

import (
	"fmt"
	"strings"
)

// NameofNameOfNamespaceCreateBySpace returns the name of the Namespace object
// that is created by the SpaceController for each Space object.
func NameOfNamespaceCreateBySpace(organizationName, spaceName string) string {
	return fmt.Sprintf("%s-%s-space", organizationName, spaceName)
}

// ComputeSpacesNamespaceFromOrganizationName returns the name of the Namespace
// where all the Space objects of the given Organization are going to be created.
func ComputeSpacesNamespaceFromOrganizationName(organization string) string {
	return organization + "-spaces"
}

// CompuComputeOrganizationNameFromSpaceNamespace returns the name of the
// Organization that owns the Space.
func ComputeOrganizationNameFromSpaceNamespace(namespace string) (string, error) {
	if strings.HasSuffix(namespace, "-spaces") {
		return strings.TrimSuffix(namespace, "-spaces"), nil
	}
	return "", fmt.Errorf("Unrecognized format %s", namespace)
}
