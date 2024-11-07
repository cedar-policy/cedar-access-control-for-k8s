package entities

import (
	"fmt"

	"k8s.io/apiserver/pkg/authorization/authorizer"
)

func ResourceRequestToPath(attributes authorizer.Attributes) string {
	base := "/api"
	if attributes.GetAPIGroup() != "" {
		base = "/apis/" + attributes.GetAPIGroup()
	}
	namespace := ""
	if attributes.GetNamespace() != "" {
		namespace = "/namespaces/" + attributes.GetNamespace()
	}
	resp := fmt.Sprintf("%s/%s%s/%s", base, attributes.GetAPIVersion(), namespace, attributes.GetResource())
	if attributes.GetName() != "" {
		resp = resp + "/" + attributes.GetName()
	}
	if attributes.GetSubresource() != "" {
		resp = resp + "/" + attributes.GetSubresource()
	}

	// TODO: implement field and label selectors?
	// If we're never adding both a filtered request and an unfiltered reqeust in the entity list,
	// it probably doesn't matter
	return resp
}
