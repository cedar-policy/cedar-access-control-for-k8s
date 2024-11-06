# Cedar Access Control for Kubernetes

<img src="docs/img/cedar-for-k8s.png#gh-light-mode-only" alt="logo" width="600"/>
<img src="docs/img/cedar-for-k8s-dark.png#gh-dark-mode-only" alt="logo" width="600"/>

[![Go Reference](https://pkg.go.dev/badge/github.com/awslabs/cedar-access-control-for-k8s.svg)](https://pkg.go.dev/github.com/awslabs/cedar-access-control-for-k8s)

## Overview

This project allows users to enforce access control on Kubernetes API requests using [Cedar policies](https://cedarpolicy.com/en).
Users can dynamically create authorization policies for Kubernetes that support features like request or user attribute based rules, label-based access controls, conditions, and enforce denial policies.
Users can also create admission policies in the same file as authorization policy, giving policy authors a single language to write and reason about.

```cedar
// Authorization cedar policy to create a secret. Create authorization requests
// do not contain the resource's name
permit (
    principal in k8s::Group::"personal-secret-creators",
    action == k8s::Action::"create",
    resource is k8s::Resource
) when {
    resource.apiGroup == "" && // "" is the core API group in Kubernetes
    resource.resource == "secret"
};

// Authorization cedar policy permitting actions on a secret that match a users's name
permit (
    principal is k8s::User,
    action in [k8s::Action::"get", k8s::Action::"update", k8s::Action::"delete"],
    resource is k8s::Resource
) when {
    principal in k8s::Group::"personal-secret-creators" &&
    resource.resource == "secret" &&
    resource.apiGroup == "" &&
    resource has name &&
    resource.name == principal.name
};

// Admission policy enforcing that a secret name in create requests match the user's name
forbid (
    principal is k8s::User,
    action == k8s::admission::Action::"create",
    resource is core::v1::Secret
) when {
    principal in k8s::Group::"personal-secret-creators"
} unless {
    // forbid doesn't apply under these conditions
    resource.metadata.name == principal.name
};
```

## Documentation

| Title | Description |
| - | - |
| [Setup](./docs/Setup.md) | A quick start guide to running this project in a local Kind cluster |
| [Cedar Introduction](./docs/CedarIntroduction.md) | An introduction on Cedar policies and request evaluation |
| [Cedar Schemas](./docs/CedarSchemas.md) | An overview of the Cedar structures used in policies in this project |
| [Demonstration](./docs/Demo.md) | A walkthrough using Cedar access controls for Kubernetes |
| [RBAC Converter](./docs/ConvertRBAC.md) | A quick demo on how to convert Kubernetes RBACs on Cedar Policies |
| [Limitations](./docs/Limitations.md) | A list of limitations of Cedar access control for Kubernetes |
| [Potential Features](./docs/FutureFeatures.md) | A list of potential features to add to this project |
| [Rejected Features](./docs/RejectedFeatures.md) | Features that were tried but failed to work out |
| [Development](./docs/Development.md) | A guide to developing and contributing to this project |

## FAQ

1. **How does Cedar differ from Kubernetes RBAC?**

    Kubernetes [Role Based Access Control][rbac] (RBAC) is an built-in authorization policy framework used to authorize Kubernetes requests.
    With RBAC, you define a policy (`ClusterRole` or `Role`) that enumerates what API groups, resources, and verbs are permitted.
    You then define a binding (`ClusterRoleBinding` or `RoleBinding`) that associates Users, Groups, or ServiceAccounts to one of those policies.
    RBAC is allow-only (no denials), and is suited for authorizing clients that need to access either specifically named resources, or whole sets of resources.

    Cedar enables you to define policies that can reference the requester or attributes of the request, conditions, and also supports denials.
    Because Cedar works on admission too, Cedar admission policies can evaluate the full request.

2. **What is Cedar?**

    Cedar is an open source policy language for defining permissions as policies, which describe who should have access to what.
    It is also a specification for evaluating those policies.
    Cedar policies can control what each user is permitted to do and what resources they may access.

    Cedar is fast and scalable.
    The policy structure is designed to be indexed for quick retrieval and to support fast and scalable real-time evaluation, with bounded latency.

    Cedar is designed for analysis using Automated Reasoning.
    This enables analyzer tools capable of optimizing your policies and proving that your security model is what you believe it is.

3. **Do I need to have an AWS account to use this?**

    No, you can try this out locally with [kind] today, and even run it on a cloud cluster that you manage.

4. **Can I use this in production?**

   While Cedar is a production-ready policy language, this project is still in active development and not yet intended for production use.
   The Kubernetes Custom Resource Definitions (CRDs) and Cedar schemas used in this project are not solidified and highly subject to change.
   If you think you've found a security issue, [please let AWS know][aws security reporting]!

5. **Can I use Cedar for Kubernetes policy enforcement?**

    While Cedar offers powerful authorization guarantees, there are policy enforcement requirements common to Kubernetes that are not [formally analyzable][analyzable].
    An example use case that illustrates this is an enforcment that all containers in all pods in a cluster have maximum memory limit set.
    Cedar is powered by automated reasoning, including an [SMT solver], which does not implement loops or map functions.
    Rather than viewing Cedar as a replacement for admission restrictions tools like [Open Policy Agent/Gatekeeper][gatekeeper] or [Kyverno][kyverno], it is best seen as an additional tool for access control enforcement.

6. **Will this be built into Amazon Elasitc Kubernetes Service (EKS)?**

    This project is a public experiment, and not currently integrated into Amazon EKS.
    We welcome your feedback, want to know what does or doesn't work for your use cases, and whether [you'd like to see this integrated into Amazon EKS][containers-roadmap].

[kind]: https://kind.sigs.k8s.io/
[rbac]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[analyzable]: https://www.amazon.science/blog/how-we-built-cedar-with-automated-reasoning-and-differential-testing
[SMT solver]: https://en.wikipedia.org/wiki/Satisfiability_modulo_theories
[gatekeeper]: https://open-policy-agent.github.io/gatekeeper/website/
[kyverno]: https://kyverno.io/
[containers-roadmap]: https://github.com/aws/containers-roadmap/issues/2463

## Community Participation

Development and discussion is coordinated in the [Cedar Slack (invite link)][cedar-slack] channel [#cedar-for-k8s][cedar-for-k8s].

Community Meetings are monthly on the second Tuesday of the month at 8:00am PT (UTC-8), 11:00pm ET (UTC-5), 17:00 CET (UTC+1).
The first meeting is December 10th, 2024.

* Amazon Chime Meeting Link: https://chime.aws/9830820801
* You can [download Amazon Chime][chime download] desktop or mobile apps, or use the web client in Google Chrome.

[cedar-slack]: https://communityinviter.com/apps/cedar-policy/cedar-policy-language
[cedar-for-k8s]: https://cedar-policy.slack.com/archives/C0802SC27UY
[chime download]: https://aws.amazon.com/chime/download-chime/?p=pm&c=ch&z=1

## Security

If you think you've found a security issue, [please let AWS know][aws security reporting]!

[aws security reporting]: https://aws.amazon.com/security/vulnerability-reporting/

## License

[Apache 2.0](LICENSE)
