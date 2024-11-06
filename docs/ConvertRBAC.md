# Convert RBAC policies

There's an RBAC converter that works on ClusterRoleBindings or RoleBindings.

If not done already, clone this repository to your local environment or IDE.

```bash
git clone https://github.com/awslabs/cedar-access-control-for-k8s.git
cd cedar-access-control-for-k8s
```

You can convert all CRBs/RBs by specifing a type with no names, or a comma-separated list of names after the type.
You can add `--output=crd` to emit Policy CRD YAML containing the cedar policies.

```bash
./bin/converter clusterrolebinding --format cedar > all-crb.cedar
./bin/converter clusterrolebinding --format crd > all-crb.yaml

./bin/converter rolebinding --format cedar > all-rb.cedar
./bin/converter rolebinding --format crd > all-rb.yaml
```

Which yields

```cedar
// cluster-admin
@clusterRoleBinding("cluster-admin")
@clusterRole("cluster-admin")
@policyRule("01")
permit (
  principal in k8s::Group::"system:masters",
  action,
  resource is k8s::NonResourceURL
);

@clusterRoleBinding("cluster-admin")
@clusterRole("cluster-admin")
@policyRule("00")
permit (
  principal in k8s::Group::"system:masters",
  action,
  resource is k8s::Resource
);
// ...
```
