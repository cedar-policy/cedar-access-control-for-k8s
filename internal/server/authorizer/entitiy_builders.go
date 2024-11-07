package authorizer

import (
	"slices"
	"strings"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/server/entities"
	cedartypes "github.com/cedar-policy/cedar-go/types"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

func ActionEntities(verb string) (cedartypes.EntityUID, cedartypes.Entities) {
	resp := cedartypes.Entities{}
	action := cedartypes.Entity{
		UID: cedartypes.EntityUID{
			Type: schema.AuthorizationActionEntityType,
			ID:   cedartypes.String(verb),
		},
	}
	if slices.Contains([]string{"get", "list", "watch"}, verb) {
		readOnlyEntityUID := cedartypes.EntityUID{
			Type: schema.AuthorizationActionEntityType,
			ID:   cedartypes.String("readOnly"),
		}
		resp[readOnlyEntityUID] = &cedartypes.Entity{UID: readOnlyEntityUID}
		action.Parents = []cedartypes.EntityUID{readOnlyEntityUID}
		resp[action.UID] = &action
	}

	return action.UID, resp
}

// inferred from https://github.com/kubernetes/kubernetes/blob/v1.31.1/staging/src/k8s.io/apiserver/pkg/endpoints/filters/impersonation.go#L84-L110
func ImpersonatedResourceToCedarEntity(attributes authorizer.Attributes) cedartypes.Entity {
	respAttributes := cedartypes.RecordMap(map[cedartypes.String]cedartypes.Value{})
	var uid cedartypes.EntityUID
	switch attributes.GetResource() {
	case "serviceaccounts":
		uid = cedartypes.EntityUID{
			Type: schema.ServiceAccountEntityType,
			ID:   cedartypes.String("system:serviceaccount:" + attributes.GetNamespace() + ":" + attributes.GetName()),
		}
		respAttributes[cedartypes.String("name")] = cedartypes.String(attributes.GetName())
		respAttributes[cedartypes.String("namespace")] = cedartypes.String(attributes.GetNamespace())
	case "uids":
		uid = cedartypes.EntityUID{
			Type: schema.PrincipalUIDEntityType,
			ID:   cedartypes.String(attributes.GetName()),
		}
	case "users":
		principalEntityType := schema.UserEntityType
		respAttributes[cedartypes.String("name")] = cedartypes.String(attributes.GetName())

		// K8s doesn't use a separate resource for node impersonation
		// https://github.com/kubernetes/kubernetes/blob/v1.31.1/staging/src/k8s.io/apiserver/pkg/endpoints/filters/impersonation.go#L84-L110
		if strings.HasPrefix(attributes.GetName(), "system:node:") && strings.Count(attributes.GetName(), ":") == 2 {
			principalEntityType = schema.NodeEntityType
			respAttributes[cedartypes.String("name")] = cedartypes.String(strings.Split(attributes.GetName(), ":")[2])
		}

		uid = cedartypes.EntityUID{
			Type: principalEntityType,
			ID:   cedartypes.String(attributes.GetName()),
		}
	case "groups":
		uid = cedartypes.EntityUID{
			Type: schema.GroupEntityType,
			ID:   cedartypes.String(attributes.GetName()),
		}
		respAttributes[cedartypes.String("name")] = cedartypes.String(attributes.GetName())
	// TODO: ENTITY TAGS: Migrate to entity tags
	case "userextras":
		uid = cedartypes.EntityUID{
			Type: schema.ExtraValuesEntityType,
			ID:   cedartypes.String(attributes.GetSubresource()),
		}
		respAttributes[cedartypes.String("key")] = cedartypes.String(attributes.GetSubresource())
		respAttributes[cedartypes.String("values")] = cedartypes.NewSet([]cedartypes.Value{cedartypes.String(attributes.GetName())})
	}
	return cedartypes.Entity{
		UID:        uid,
		Attributes: cedartypes.NewRecord(respAttributes),
	}
}

func NonResourceToCedarEntity(attributes authorizer.Attributes) cedartypes.Entity {
	return cedartypes.Entity{
		UID: cedartypes.EntityUID{
			Type: schema.NonResourceURLEntityType,
			ID:   cedartypes.String(attributes.GetPath()),
		},
		Attributes: cedartypes.NewRecord(map[cedartypes.String]cedartypes.Value{
			cedartypes.String("path"): cedartypes.String(attributes.GetPath()),
		}),
	}
}

func ResourceToCedarEntity(attributes authorizer.Attributes) cedartypes.Entity {
	respAttributes := cedartypes.RecordMap(map[cedartypes.String]cedartypes.Value{})

	respAttributes[cedartypes.String("apiGroup")] = cedartypes.String(attributes.GetAPIGroup())
	respAttributes[cedartypes.String("resource")] = cedartypes.String(attributes.GetResource())
	if attributes.GetName() != "" {
		respAttributes[cedartypes.String("name")] = cedartypes.String(attributes.GetName())
	}
	if attributes.GetSubresource() != "" {
		respAttributes[cedartypes.String("subresource")] = cedartypes.String(attributes.GetSubresource())
	}
	if attributes.GetNamespace() != "" {
		respAttributes[cedartypes.String("namespace")] = cedartypes.String(attributes.GetNamespace())
	}
	if labelSelector, err := attributes.GetLabelSelector(); err == nil && len(labelSelector) > 0 {
		selectors := []cedartypes.Value{}
		for _, selector := range labelSelector {
			values := []cedartypes.Value{}
			for v := range selector.Values() {
				values = append(values, cedartypes.String(v))
			}
			selectors = append(selectors, cedartypes.NewRecord(map[cedartypes.String]cedartypes.Value{
				"key":      cedartypes.String(selector.Key()),
				"operator": cedartypes.String(selector.Operator()),
				"values":   cedartypes.NewSet(values),
			}))
		}
		respAttributes[cedartypes.String("labelSelector")] = cedartypes.NewSet(selectors)
	} else if err != nil {
		klog.Error("error parsing label selector", "error", err)
	}

	if fieldSelector, err := attributes.GetFieldSelector(); err == nil && len(fieldSelector) > 0 {
		selectors := []cedartypes.Value{}
		for _, selector := range fieldSelector {
			selectors = append(selectors, cedartypes.NewRecord(map[cedartypes.String]cedartypes.Value{
				"field":    cedartypes.String(selector.Field),
				"operator": cedartypes.String(selector.Operator),
				"value":    cedartypes.String(selector.Value),
			}))
		}
		respAttributes[cedartypes.String("fieldSelector")] = cedartypes.NewSet(selectors)
	} else if err != nil {
		klog.Error("error parsing field selector", "error", err)
	}

	return cedartypes.Entity{
		UID: cedartypes.EntityUID{
			Type: schema.ResourceEntityType,
			ID:   cedartypes.String(entities.ResourceRequestToPath(attributes)),
		},
		Attributes: cedartypes.NewRecord(respAttributes),
		// TODO: Parent of Namespace Entity for namespaced resources?
		// maybe the best argument for a namespaced resource
		// or everything has a namespace parent of "all"?
	}
}
