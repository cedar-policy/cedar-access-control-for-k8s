# Cedar access controls for Kubernetes

## Demo

### Setup

This demo assumes you've already followed the setup instructions, installed both the authorizer and admission webhook as described in [docs/Setup.md](Setup.md), and have the test-user `kubeconfig` created.

You can validate that the `test-user` can access the cluster by running a `whoami`.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl auth whoami
# ATTRIBUTE   VALUE
# Username    test-user
# Groups      [viewers test-group system:authenticated]
```

### Authorization

#### Basic access

We have a couple of policies already written on the [`demo/authorization-policy.yaml`](../demo/authorization-policy.yaml) file for our `test-user`. These were already applied during the setup process.

```cedar
@description("test-user can get/list/watch pods")
permit (
    principal is k8s::User,
    action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
    resource is k8s::Resource
) when {
    principal.name == "test-user" &&
    // "" is the core API group in Kubernetes
    resource.apiGroup == "" &&
    resource.resource == "pods"
};

@description("forbid test-user to get/list/watch nodes")
forbid (
    principal is k8s::User,
    action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
    resource is k8s::Resource
) when {
    principal.name == "test-user" &&
    // "" is the core API group in Kubernetes
    resource.apiGroup == "" &&
    resource.resource == "nodes"
};
```

Try getting resources.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get pods -A # allowed
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get nodes # denied
```

While we could equivalently express the first policy shown above in RBAC, the second rule is not possible as RBAC does not support denials.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: basic-rule
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: basic-rule
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: basic-rule
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: test-user
```

#### Group access

Given that the test-user is in the group `viewers`, lets leverage that group by looking at the policy written for it:

```cedar
// viewer group members can get/list/watch any Resource other than secrets in the default namespace
permit (
    principal in k8s::Group::"viewers",
    action in [ k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
    resource is k8s::Resource
) unless {
    resource.resource == "secrets" &&
    resource has namespace &&
    resource.namespace == "default" &&
    // "" is the core API group in Kubernetes
    resource.apiGroup == ""
};
```

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get serviceaccounts # allowed
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get secrets # denied
```

#### Attribute-based access control

Kubernetes `v1.31` added alpha support for evaluating labels and field selectors in authorization.
One feature of Cedar is that conditions can not only reference the resource or principal of a policy, they can reference both on opposite sides of an operator.
Let's try it out! The policy looks like this.

```cedar
// We allow users to list and watch secrets when the request
// includes the label selector owner={request.user.name}
permit (
    principal is k8s::User,
    action in [k8s::Action::"list", k8s::Action::"watch"],
    resource is k8s::Resource
) when {
    resource.resource == "secrets" &&
    resource.apiGroup == "" &&
    resource has labelSelector &&
    resource.labelSelector.contains({
      "key": "owner",
      "operator": "=",
      "values": [principal.name]
    })
};
```

As `cluster-dmin` we can see all secrets and display their labels

```bash
kubectl get secrets --show-labels
```

Try to list secrets as the test-user

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get secrets # denied
```

Try to list secrets as the test user using a label selector

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get secrets -l owner=test-user --show-labels # allowed
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get secrets -l owner=other-user --show-labels # denied
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get secrets -l 'owner in (test-user)' --show-labels # allowed
```

#### Impersonation

Kubernetes supports [impersonation][k8s-impersonation], where you can act as a different user or group using a special header by specifying the `--as` or `--as-group` flag in `kubectl`. Let's see a Cedar policy for this:

[k8s-impersonation]: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation

```cedar
// test-user can impersonate the service-manager service account in the default namespace
permit (
    principal is k8s::User,
    action in [k8s::Action::"impersonate"],
    resource is k8s::ServiceAccount
) when {
    principal.name == "test-user" &&
    resource has namespace &&
    resource.namespace == "default" &&
    resource has name &&
    resource.name == "service-manager"
};

// SA named 'service-manager' can act on services in its own namespace
permit (
    principal is k8s::ServiceAccount,
    action,
    resource is k8s::Resource
) when {
    principal.name == "service-manager" && // no principal.namespace restriction
    resource.resource == "services" &&
    resource has namespace &&
    resource.namespace == principal.namespace
};
```

Check if you can impersonate the service account service-manager.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl auth can-i impersonate serviceaccount/service-manager
# yes
```

Check if you can create a service.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl auth can-i create service
# no
```

Impersonate a request as the service-manager.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get configmap --as system:serviceaccount:default:service-manager # denied
```

Check if you can create a service as the service-manager.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl auth can-i create service --as system:serviceaccount:default:service-manager
# yes
```

Create a service.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl --as system:serviceaccount:default:service-manager create service clusterip my-cool-service --tcp 80:80 # allowed
```

### Admission

During the setup, we already applied a few admission policies written on the file [`demo/admission-policy.yaml](../demo/admission-policy.yaml). If you made changes on this file, run the following to apply the customized policies.

```bash
kubectl apply -f demo/admission-policy.yaml
```

Cedar's default decision is to deny requests if there are no matching `permit` or `forbid` policies, and that explicit `forbid`s have precedence over any matching `permit`. This works well for authorization, but less so for admission.  
Because requests have already been authorized, we want to allow them by default without the user having to say so.  
The admission webhook automatically applies a policy that allows all admission requests, so users only need to write `forbid` policies.

```cedar
@description("default allow-all admission rule")
permit (
  principal,
  action in [
    k8s::admission::Action::"create",
    k8s::admission::Action::"update",
    k8s::admission::Action::"delete",
    k8s::admission::Action::"connect"],
  resource
);
```

#### Name selection

Because all policies are written in Cedar, we can write both authentication and authorization policies in the same language and even same file.

```cedar
// Authorization policy
// test-user can do Action::"*" on configmaps in the default namespace
permit (
    principal is k8s::User,
    action,
    resource is k8s::Resource
) when {
    principal.name == "test-user" &&
    resource has namespace &&
    resource.namespace == "default" &&
    // "" is the core API group in Kubernetes
    resource.apiGroup == "" &&
    resource.resource == "configmaps"
};

// Admission policy preventing test-user from creating/updating configmaps with name starting with "prod"
forbid (
    principal is k8s::User,
    action in [k8s::admission::Action::"create", k8s::admission::Action::"update"],
    resource is core::v1::ConfigMap
) when {
    principal.name == "test-user" &&
    resource has metadata &&
    resource.metadata has name &&
    resource.metadata.name like "prod*"
};
```

Let's test this out.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get configmap
```

Try creating a ConfigMap.

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl create configmap test-config --from-literal=stage=test
```

Try creating with a name starting with "prod"

```bash
KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl create configmap prod-config --from-literal=stage=prod
```

These types of admission rules are already possible today with general purpose tools like Open Policy Agent (OPA) or Kyverno, but each requires learning and writing separate domain languages than what authorization policies are defined in (RBAC).

#### Key/Value maps

For now, the webhook modifies labels and annotations into a list of structure with the fields "key" and "value", so policy can be written against it.
Once cedar-go [adds support for entity tags][cedar-go-entity-tags], we'll refactor to use that structure and syntax.
That will be a breaking change to any policy using the current key/value structure.

[cedar-go-entity-tags]: https://github.com/cedar-policy/cedar-go/issues/47

```cedar
// authorization policy allowing sample-user to make actions on configmaps in default namespace
// sample-user is in the `requires-labels` group
permit (
    principal is k8s::User,
    action in [
        k8s::Action::"create",
        k8s::Action::"list",
        k8s::Action::"watch",
        k8s::Action::"update",
        k8s::Action::"patch",
        k8s::Action::"delete"],
    resource is k8s::Resource
) when {
    principal.name == "sample-user" &&
    resource has namespace &&
    resource.namespace == "default" &&
    // "" is the core API group in Kubernetes
    resource.apiGroup == "" &&
    resource.resource == "configmaps"
};

// authz policy forbiding users in group "requires-labels" from making list/watches
// without label selector owner={principal.name}
forbid (
    principal is k8s::User,
    action in [k8s::Action::"list", k8s::Action::"watch"],
    resource is k8s::Resource
) when {
    principal in k8s::Group::"requires-labels"
} unless {
    resource has labelSelector &&
    resource.labelSelector.contains({
      "key": "owner",
      "operator": "=",
      "values": [principal.name]
    })
};

// admission policy to forbid resource creation without an owner key
forbid (
    principal is k8s::User,
    action in [k8s::admission::Action::"create", k8s::admission::Action::"update"],
    resource
) when {
    principal in k8s::Group::"requires-labels"
} unless {
    resource has metadata &&
    resource.metadata has labels &&
    resource.metadata.labels.contains({"key": "owner", "value": principal.name})
};

// admission policy forbidding users in "requires-labels" group from updating or deleting
// resources that they don't already have the owner={principal.name} label
forbid (
    principal is k8s::User,
    action in [k8s::admission::Action::"delete", k8s::admission::Action::"update"],
    resource
) when {
    principal in k8s::Group::"requires-labels"
} unless {
    context has oldObject &&
    context.oldObject has metadata &&
    context.oldObject.metadata has labels &&
    context.oldObject.metadata.labels.contains({"key": "owner", "value": principal.name})
};
```

Let's try this as well.

As `cluster-admin`

```bash
kubectl create configmap other-config --from-literal=foo=bar
kubectl label cm/other-config owner=some-user
kubectl get cm --show-labels
```

Validate the `sample-user`.

```bash
KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl auth whoami
# ATTRIBUTE   VALUE
# Username    sample-user
# Groups      [sample-group requires-labels system:authenticated]
```

Try to list ConfigMaps, or modify a ConfigMap as `sample-user`

```bash
KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl get cm # denied
KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl label cm/other-config stage=test # denied
```

Try to create a new ConfigMap.

```bash
KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl create cm sample-config --from-literal=k1=v1 # allowed
```

Now, try to create a ConfigMaps with the label `owner=sample-user`, and list the ConfigMaps with that specific label.

```bash
cat << EOF | KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl create -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: sample-config
  labels:
    owner: sample-user
data:
  stage: test
EOF
KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl get cm -l owner=sample-user --show-labels # allowed
```

Try to change the owner of that ConfigMap.

```bash
KUBECONFIG=./mount/sample-user-kubeconfig.yaml kubectl patch configmap/sample-config -p '{"metadata":{"labels":{"owner":"other-user"}}}' # denied
```
