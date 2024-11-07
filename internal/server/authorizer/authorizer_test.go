package authorizer

import (
	"context"
	"testing"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/server/options"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/server/store"
	"github.com/cedar-policy/cedar-go"
	cedartypes "github.com/cedar-policy/cedar-go/types"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

var (
	listAction = cedartypes.Entity{
		UID:     cedartypes.EntityUID{Type: schema.AuthorizationActionEntityType, ID: "list"},
		Parents: []cedartypes.EntityUID{{Type: schema.AuthorizationActionEntityType, ID: "readOnly"}},
	}
	getAction = cedartypes.Entity{
		UID:     cedartypes.EntityUID{Type: schema.AuthorizationActionEntityType, ID: "get"},
		Parents: []cedartypes.EntityUID{{Type: schema.AuthorizationActionEntityType, ID: "readOnly"}},
	}
	readOnlyAction = cedartypes.Entity{
		UID: cedartypes.EntityUID{Type: schema.AuthorizationActionEntityType, ID: "readOnly"},
	}
)

func TestRecordToCedarResource(t *testing.T) {
	cases := []struct {
		name         string
		input        authorizer.Attributes
		wantEntities cedartypes.Entities
		wantRequest  cedar.Request
	}{
		{
			"Resource with namespace",
			authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			cedartypes.Entities{
				getAction.UID: &getAction,
				cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"}: {
					UID: cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"},
					Parents: []cedartypes.EntityUID{
						{Type: schema.GroupEntityType, ID: "test-group"},
					},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"name": cedartypes.String("test-user"),
						"extra": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap{
								"key": cedartypes.String("attr1"),
								"values": cedartypes.NewSet([]cedartypes.Value{
									cedartypes.String("value1"),
								}),
							}),
						}),
					}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("test-group")}),
				},
				cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/namespaces/default/pods/test-pod"}: {
					UID: cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/namespaces/default/pods/test-pod"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"apiGroup":  cedartypes.String(""),
						"namespace": cedartypes.String("default"),
						"resource":  cedartypes.String("pods"),
						"name":      cedartypes.String("test-pod"),
					}),
				},
				readOnlyAction.UID: &readOnlyAction,
			},
			cedartypes.Request{
				Principal: cedartypes.EntityUID{
					Type: schema.UserEntityType,
					ID:   "1234567890",
				},
				Action: cedartypes.EntityUID{
					Type: schema.AuthorizationActionEntityType,
					ID:   "get",
				},
				Resource: cedartypes.EntityUID{
					Type: schema.ResourceEntityType,
					ID:   "/api/v1/namespaces/default/pods/test-pod",
				},
			},
		},
		{
			string(schema.ResourceEntityType),
			authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "list",
				Namespace:       "", // cluster scoped list
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "",
				ResourceRequest: true,
				Path:            "",
			},
			cedartypes.Entities{
				listAction.UID:     &listAction,
				readOnlyAction.UID: &readOnlyAction,
				cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"}: {
					UID: cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"},
					Parents: []cedartypes.EntityUID{
						{Type: schema.GroupEntityType, ID: "test-group"},
					},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"name": cedartypes.String("test-user"),
						"extra": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap{
								"key": cedartypes.String("attr1"),
								"values": cedartypes.NewSet([]cedartypes.Value{
									cedartypes.String("value1"),
								}),
							}),
						}),
					}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("test-group")}),
				},
				cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/pods"}: {
					UID: cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/pods"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"apiGroup": cedartypes.String(""),
						"resource": cedartypes.String("pods"),
					}),
				},
			},
			cedartypes.Request{
				Principal: cedartypes.EntityUID{
					Type: schema.UserEntityType,
					ID:   "1234567890",
				},
				Action: cedartypes.EntityUID{
					Type: schema.AuthorizationActionEntityType,
					ID:   "list",
				},
				Resource: cedartypes.EntityUID{
					Type: schema.ResourceEntityType,
					ID:   "/api/v1/pods",
				},
			},
		},
		{
			"NonResourceURL",
			authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "get",
				Namespace:       "",
				APIGroup:        "",
				APIVersion:      "",
				Resource:        "",
				Subresource:     "",
				Name:            "",
				ResourceRequest: false,
				Path:            "/metrics",
			},
			cedartypes.Entities{
				getAction.UID:      &getAction,
				readOnlyAction.UID: &readOnlyAction,
				cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"}: {
					UID: cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"},
					Parents: []cedartypes.EntityUID{
						{Type: schema.GroupEntityType, ID: "test-group"},
					},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"name": cedartypes.String("test-user"),
						"extra": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap{
								"key": cedartypes.String("attr1"),
								"values": cedartypes.NewSet([]cedartypes.Value{
									cedartypes.String("value1"),
								}),
							}),
						}),
					}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("test-group")}),
				},
				cedartypes.EntityUID{Type: schema.NonResourceURLEntityType, ID: "/metrics"}: {
					UID:        cedartypes.EntityUID{Type: schema.NonResourceURLEntityType, ID: "/metrics"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"path": cedartypes.String("/metrics")}),
				},
			},
			cedartypes.Request{
				Principal: cedartypes.EntityUID{
					Type: schema.UserEntityType,
					ID:   "1234567890",
				},
				Action: cedartypes.EntityUID{
					Type: schema.AuthorizationActionEntityType,
					ID:   "get",
				},
				Resource: cedartypes.EntityUID{
					Type: schema.NonResourceURLEntityType,
					ID:   "/metrics",
				},
			},
		},
		{
			"apigroup subresource",
			authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "patch",
				Namespace:       "default",
				APIGroup:        "apps",
				APIVersion:      "v1",
				Resource:        "deployments",
				Subresource:     "scale",
				Name:            "nginx",
				ResourceRequest: true,
				Path:            "",
			},
			cedartypes.Entities{
				cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"}: {
					UID: cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"},
					Parents: []cedartypes.EntityUID{
						{Type: schema.GroupEntityType, ID: "test-group"},
					},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"name": cedartypes.String("test-user"),
						"extra": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap{
								"key": cedartypes.String("attr1"),
								"values": cedartypes.NewSet([]cedartypes.Value{
									cedartypes.String("value1"),
								}),
							}),
						}),
					}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("test-group")}),
				},
				cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/apis/apps/v1/namespaces/default/deployments/nginx/scale"}: {
					UID: cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/apis/apps/v1/namespaces/default/deployments/nginx/scale"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"apiGroup":    cedartypes.String("apps"),
						"namespace":   cedartypes.String("default"),
						"resource":    cedartypes.String("deployments"),
						"name":        cedartypes.String("nginx"),
						"subresource": cedartypes.String("scale"),
					}),
				},
			},
			cedartypes.Request{
				Principal: cedartypes.EntityUID{
					Type: schema.UserEntityType,
					ID:   "1234567890",
				},
				Action: cedartypes.EntityUID{
					Type: schema.AuthorizationActionEntityType,
					ID:   "patch",
				},
				Resource: cedartypes.EntityUID{
					Type: schema.ResourceEntityType,
					ID:   "/apis/apps/v1/namespaces/default/deployments/nginx/scale",
				},
			},
		},
		{
			"ServiceAccount principal",
			authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "system:serviceaccount:foo:bar",
					Groups: []string{"system:serviceaccounts", "system:serviceaccounts:default", "system:authenticated"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			cedartypes.Entities{
				getAction.UID:      &getAction,
				readOnlyAction.UID: &readOnlyAction,
				cedartypes.EntityUID{Type: schema.ServiceAccountEntityType, ID: "1234567890"}: {
					UID: cedartypes.EntityUID{Type: schema.ServiceAccountEntityType, ID: "1234567890"},
					Parents: []cedartypes.EntityUID{
						{Type: schema.GroupEntityType, ID: "system:serviceaccounts"},
						{Type: schema.GroupEntityType, ID: "system:serviceaccounts:default"},
						{Type: schema.GroupEntityType, ID: "system:authenticated"},
					},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"name":      cedartypes.String("bar"),
						"namespace": cedartypes.String("foo"),
						"extra": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap{
								"key": cedartypes.String("attr1"),
								"values": cedartypes.NewSet([]cedartypes.Value{
									cedartypes.String("value1"),
								}),
							}),
						}),
					}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "system:serviceaccounts"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "system:serviceaccounts"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("system:serviceaccounts")}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "system:serviceaccounts:default"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "system:serviceaccounts:default"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("system:serviceaccounts:default")}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "system:authenticated"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "system:authenticated"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("system:authenticated")}),
				},
				cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/namespaces/default/pods/test-pod"}: {
					UID: cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/namespaces/default/pods/test-pod"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"apiGroup":  cedartypes.String(""),
						"namespace": cedartypes.String("default"),
						"resource":  cedartypes.String("pods"),
						"name":      cedartypes.String("test-pod"),
					}),
				},
			},
			cedartypes.Request{
				Principal: cedartypes.EntityUID{
					Type: schema.ServiceAccountEntityType,
					ID:   "1234567890",
				},
				Action: cedartypes.EntityUID{
					Type: schema.AuthorizationActionEntityType,
					ID:   "get",
				},
				Resource: cedartypes.EntityUID{
					Type: schema.ResourceEntityType,
					ID:   "/api/v1/namespaces/default/pods/test-pod",
				},
			},
		},
		{
			"labelSelector & fieldSelector",
			authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:            "list",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "",
				ResourceRequest: true,
				Path:            "",
				LabelSelectorRequirements: labels.Requirements{
					func() labels.Requirement {
						resp, _ := labels.NewRequirement("owner", selection.Equals, []string{"test-user"})
						return *resp
					}(),
				},
				FieldSelectorRequirements: fields.Requirements{
					fields.Requirement{
						Field:    ".spec.nodeName",
						Operator: selection.Equals,
						Value:    "test-node",
					},
				},
			},
			cedartypes.Entities{
				listAction.UID:     &listAction,
				readOnlyAction.UID: &readOnlyAction,
				cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"}: {
					UID: cedartypes.EntityUID{Type: schema.UserEntityType, ID: "1234567890"},
					Parents: []cedartypes.EntityUID{
						{Type: schema.GroupEntityType, ID: "test-group"},
					},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"name": cedartypes.String("test-user"),
						"extra": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap{
								"key": cedartypes.String("attr1"),
								"values": cedartypes.NewSet([]cedartypes.Value{
									cedartypes.String("value1"),
								}),
							}),
						}),
					}),
				},
				cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"}: {
					UID:        cedartypes.EntityUID{Type: schema.GroupEntityType, ID: "test-group"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{"name": cedartypes.String("test-group")}),
				},
				cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/namespaces/default/pods"}: {
					UID: cedartypes.EntityUID{Type: schema.ResourceEntityType, ID: "/api/v1/namespaces/default/pods"},
					Attributes: cedartypes.NewRecord(cedartypes.RecordMap{
						"apiGroup":  cedartypes.String(""),
						"namespace": cedartypes.String("default"),
						"resource":  cedartypes.String("pods"),
						"labelSelector": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap(map[cedartypes.String]cedartypes.Value{
								"key":      cedartypes.String("owner"),
								"operator": cedartypes.String("="),
								"values":   cedartypes.NewSet([]cedartypes.Value{cedartypes.String("test-user")}),
							})),
						}),
						"fieldSelector": cedartypes.NewSet([]cedartypes.Value{
							cedartypes.NewRecord(cedartypes.RecordMap(map[cedartypes.String]cedartypes.Value{
								"field":    cedartypes.String(".spec.nodeName"),
								"operator": cedartypes.String("="),
								"value":    cedartypes.String("test-node"),
							})),
						}),
					}),
				},
			},
			cedartypes.Request{
				Principal: cedartypes.EntityUID{
					Type: schema.UserEntityType,
					ID:   "1234567890",
				},
				Action: cedartypes.EntityUID{
					Type: schema.AuthorizationActionEntityType,
					ID:   "list",
				},
				Resource: cedartypes.EntityUID{
					Type: schema.ResourceEntityType,
					ID:   "/api/v1/namespaces/default/pods",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotEntities, gotRequest := RecordToCedarResource(tc.input)
			if diff := cmp.Diff(gotEntities, tc.wantEntities); diff != "" {
				t.Errorf("Didn't get same entities: %s", diff)
				return
			}
			if diff := cmp.Diff(tc.wantRequest, gotRequest); diff != "" {
				t.Errorf("Didn't get same request: %s", diff)
			}
		})
	}
}

func TestAuthorize(t *testing.T) {

	cases := []struct {
		name          string
		inputPolicy   string
		input         authorizer.Attributes
		storeComplete bool
		wantDecision  authorizer.Decision
		wantReason    string
	}{
		{
			name: "Allow",
			inputPolicy: `
permit (
	principal,
	action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
	resource is k8s::Resource
) when {
	principal.name == "test-user" &&
	resource.resource == "pods"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Allow Impersonate UID",
			inputPolicy: `
permit (
	principal,
	action == k8s::Action::"impersonate",
	resource == k8s::PrincipalUID::"1234"
) when {
	principal.name == "test-user"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:            "impersonate",
				Namespace:       "",
				APIGroup:        "",
				APIVersion:      "",
				Resource:        "uids",
				Subresource:     "",
				Name:            "1234",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow Impersonate UID","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Allow Impersonate serviceaccount",
			inputPolicy: `
permit (
	principal,
	action == k8s::Action::"impersonate",
	resource is k8s::ServiceAccount
) when {
	principal.name == "test-user" &&
	resource.name == "default" &&
	resource.namespace == "kube-system"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:            "impersonate",
				Namespace:       "kube-system",
				APIGroup:        "",
				APIVersion:      "",
				Resource:        "serviceaccounts",
				Subresource:     "",
				Name:            "default",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow Impersonate serviceaccount","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Allow Impersonate serviceaccount id",
			inputPolicy: `
permit (
	principal,
	action == k8s::Action::"impersonate",
	resource == k8s::ServiceAccount::"system:serviceaccount:kube-system:default"
) when {
	principal.name == "test-user"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:            "impersonate",
				Namespace:       "kube-system",
				APIGroup:        "",
				APIVersion:      "",
				Resource:        "serviceaccounts",
				Subresource:     "",
				Name:            "default",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow Impersonate serviceaccount id","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Allow Impersonate node",
			inputPolicy: `
permit (
	principal,
	action == k8s::Action::"impersonate",
	resource is k8s::Node
) when {
	principal.name == "test-user" &&
	resource.name == "ip-10-24-34-0.us-west-2.compute.internal"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:       "impersonate",
				Namespace:  "",
				APIGroup:   "",
				APIVersion: "",
				// K8s doesn't use a separate resource for node impersonation
				// https://github.com/kubernetes/kubernetes/blob/v1.31.1/staging/src/k8s.io/apiserver/pkg/endpoints/filters/impersonation.go#L84-L110
				Resource:        "users",
				Subresource:     "",
				Name:            "system:node:ip-10-24-34-0.us-west-2.compute.internal",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow Impersonate node","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Allow Impersonate user",
			inputPolicy: `
permit (
	principal,
	action == k8s::Action::"impersonate",
	resource is k8s::User
) when {
	principal.name == "test-user" &&
	resource.name == "test-impersonated"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:            "impersonate",
				Namespace:       "",
				APIGroup:        "",
				APIVersion:      "",
				Resource:        "users",
				Subresource:     "",
				Name:            "test-impersonated",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow Impersonate user","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Allow Impersonate group",
			inputPolicy: `
permit (
	principal,
	action == k8s::Action::"impersonate",
	resource is k8s::Group
) when {
	principal.name == "test-user" &&
	resource.name == "test-impersonated-group"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{"attr1": {"value1"}},
				},
				Verb:            "impersonate",
				Namespace:       "",
				APIGroup:        "",
				APIVersion:      "",
				Resource:        "groups",
				Subresource:     "",
				Name:            "test-impersonated-group",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Allow Impersonate group","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "Explicit Deny",
			inputPolicy: `
forbid (
	principal,
	action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
	resource is k8s::Resource
) when {
	principal.name == "test-user" &&
	resource.resource == "pods"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionDeny,
			wantReason:    `{"reasons":[{"policy":"policy0","position":{"filename":"Explicit Deny","offset":1,"line":2,"column":1}}]}`,
		},
		{
			name: "No Opinion",
			inputPolicy: `
forbid (
	principal,
	action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
	resource is k8s::Resource
) when {
	principal.name == "test-user" &&
	resource.resource == "nodes"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra: map[string][]string{
						"attr1": {"value1"},
					},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionNoOpinion,
			wantReason:    ``,
		},
		{
			name: "system identity: No Opinion",
			inputPolicy: `
forbid (
	principal,
	action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
	resource is k8s::Resource
) when {
	principal.name == "test-user" &&
	resource.resource == "nodes"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "system:kube-apiserver",
					Groups: []string{"system:masters"},
					Extra:  map[string][]string{},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionNoOpinion,
			wantReason:    ``,
		},
		{
			name: "store incomplete: No Opinion",
			inputPolicy: `
forbid (
	principal,
	action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
	resource is k8s::Resource
) when {
	principal.name == "test-user" &&
	resource.resource == "nodes"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   "test-user",
					Groups: []string{"test-group"},
					Extra:  map[string][]string{},
				},
				Verb:            "get",
				Namespace:       "default",
				APIGroup:        "",
				APIVersion:      "v1",
				Resource:        "pods",
				Subresource:     "",
				Name:            "test-pod",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: false,
			wantDecision:  authorizer.DecisionNoOpinion,
			wantReason:    ``,
		},
		{
			name: "allow self",
			inputPolicy: `
forbid (
	principal,
	action in [k8s::Action::"get", k8s::Action::"list", k8s::Action::"watch"],
	resource is k8s::Resource
) when {
	principal.name == "test-user" &&
	resource.resource == "nodes"
};`,
			input: authorizer.AttributesRecord{
				User: &user.DefaultInfo{
					UID:    "1234567890",
					Name:   options.CedarAuthorizerIdentityName,
					Groups: []string{},
					Extra:  map[string][]string{},
				},
				Verb:            "list",
				Namespace:       "",
				APIGroup:        "cedar.k8s.aws",
				APIVersion:      "v1",
				Resource:        "policies",
				Subresource:     "",
				Name:            "",
				ResourceRequest: true,
				Path:            "",
			},
			storeComplete: true,
			wantDecision:  authorizer.DecisionAllow,
			wantReason:    `cedar authorizer is always allowed to access policies`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policyStore, err := store.NewMemoryStore(tc.name, []byte(tc.inputPolicy), tc.storeComplete)
			if err != nil {
				t.Errorf("Failed to create policy store: %s", err)
				return
			}
			authorizer := cedarWebhookAuthorizer{store: policyStore}
			dec, reason, err := authorizer.Authorize(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if dec != tc.wantDecision {
				t.Errorf("Didn't get same decision: got %v: wanted %v", dec, tc.wantDecision)
			}
			if reason != tc.wantReason {
				t.Errorf("Didn't get same reason: got `%v`: wanted `%v`", reason, tc.wantReason)
				return
			}
		})
	}
}
