The Organization operator is a kubernetes operator that manages two types of
kubernetes custom resources: Organization and Space.

These resources can be used to partition a kubernetes cluster into smaller ones.

## Goal

We want to have multiple tenants operating on the same kubernetes cluster.
We want to isolate these tenants by leveraging either features built into
kubernetes or by using additional components extending kubernetes.

The proposal mimics what [Cloud Foundry is doing for its multi-tenancy solution](https://docs.cloudfoundry.org/concepts/roles.html):

 * Each tenant can have one or more **Organization**
 * Each Organization can have multiple teams working on their own dedicated **Space**

The proposal assumes the following personas are going to operate on this
kubernetes infrastructure:

 * Platform admins: they are the operators of the underlying kubernetes cluster.
   They have ultimate access to all parts of it.
 * Organization users, they are divided among three groups:
    * Admins
    * Editors
    * Viewers

Note well: the admin/edit/view roles are going to be implemented using the
pre-defined ClusterRoles defined by kubernetes.
See [this section](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles)
of kubernetes’ upstream docs for more details.

The proposal requires that nobody, except for platform admins, have write
access to the kubernetes namespace objects.
Note well: that happens by default unless specific RBAC policies are created on the cluster.

## Architecture

The architecture of the Organization Controller can be find inside of
[this Google Doc](https://docs.google.com/document/d/1qHkPK3fem5oanaD35E7BC7SkdIjn8oM-F7CUKq0G5Wc/edit?usp=sharing)

Feedback on the Google doc is highly appreciated.

## Current state

This repository holds a quick POC of what is being described inside of the
architecture document.

This kubernetes operator is created using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

What is currently missing:

  * [ ] `SpaceExtraConfig` CR
  * [ ] Reconcile objects if they are changed; deleted ones are reconciled but changes are not processed right now.
  * [ ] Testing, linting
  * [ ] Deployment resources: helm charts, container image,...

Right now it's possible to experiment with the operator by performing the following steps:

  * Checkout repository
  * Have a kubernetes cluster at reach (minikube or kind are good enough)
  * Ensure you have `admin` rights on the target cluster
  * Run `make install`
  * Run `make run ENABLE_WEBHOOKS=false`
