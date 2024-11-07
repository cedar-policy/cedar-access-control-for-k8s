package authorizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/server/entities"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/server/options"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/server/store"
	"github.com/cedar-policy/cedar-go"
	cedartypes "github.com/cedar-policy/cedar-go/types"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

// Authorizer makes authorization decisions
type Authorizer interface {
	Authorize(context.Context, authorizer.Attributes) (authorizer.Decision, string, error)
}

// NewAuthorizer creates a Cedar authorizer
func NewAuthorizer(policyStore store.PolicyStore) Authorizer {
	return &cedarWebhookAuthorizer{store: policyStore}
}

type cedarWebhookAuthorizer struct {
	store store.PolicyStore
}

func (e *cedarWebhookAuthorizer) Authorize(ctx context.Context, requestAttributes authorizer.Attributes) (authorizer.Decision, string, error) {
	// Always allow self to read policies
	if requestAttributes.GetUser().GetName() == options.CedarAuthorizerIdentityName &&
		requestAttributes.IsReadOnly() &&
		requestAttributes.GetAPIGroup() == "cedar.k8s.aws" &&
		requestAttributes.GetResource() == "policies" {
		return authorizer.DecisionAllow, "cedar authorizer is always allowed to access policies", nil
	}

	if strings.HasPrefix(requestAttributes.GetUser().GetName(), "system:") &&
		!strings.HasPrefix(requestAttributes.GetUser().GetName(), "system:serviceaccount:") &&
		!strings.HasPrefix(requestAttributes.GetUser().GetName(), "system:node:") {
		// skip system users (nodes, anonymous, internal identities) for development, helps from accidentally halting normal operations
		// TODO: are there any system users we should always skip? Anonymous probably?
		return authorizer.DecisionNoOpinion, "", nil
	}

	if !e.store.InitalPolicyLoadComplete() {
		klog.V(3).Info("Policy store not yet loaded, issuing no opinion")
		return authorizer.DecisionNoOpinion, "", nil
	}
	entities, request := RecordToCedarResource(requestAttributes)
	entityJson, _ := entities.MarshalJSON()
	requestJson, _ := json.Marshal(request)
	klog.V(3).Info("Request entities ", string(entityJson))
	klog.V(3).Info("Cedar request ", string(requestJson))
	ok, diagnostic := e.store.PolicySet(ctx).IsAuthorized(entities, request)
	klog.V(9).InfoS("Authorize", "ok", ok, "Diagnostic", diagnosticToReason(diagnostic))
	if ok {
		return authorizer.DecisionAllow, diagnosticToReason(diagnostic), nil
	} else if !ok && len(diagnostic.Reasons) > 0 {
		return authorizer.DecisionDeny, diagnosticToReason(diagnostic), nil
	}
	// In the case of failure, we don't want to leave an opinion
	if len(diagnostic.Errors) > 0 {
		klog.Error("Authorize", "errors", diagnostic.Errors)
	}
	return authorizer.DecisionNoOpinion, "", nil
}

type entityDerivationFunc = func(attributes authorizer.Attributes) cedartypes.Entity

// mergeMaps merges right into left, overwriting any existing keys in left
func mergeMaps[Map ~map[K]V, K comparable, V any](left Map, right Map) {
	for k, v := range right {
		left[k] = v
	}
}

func RecordToCedarResource(attributes authorizer.Attributes) (cedartypes.Entities, cedar.Request) {
	action, reqEntities := ActionEntities(attributes.GetVerb())
	principalUID, principalEntities := entities.UserToCedarEntity(attributes.GetUser())

	req := cedar.Request{
		Principal: principalUID,
		Action:    action,
	}
	mergeMaps(reqEntities, principalEntities)

	var resourceEntityFunc entityDerivationFunc = NonResourceToCedarEntity
	if attributes.IsResourceRequest() {
		resourceEntityFunc = ResourceToCedarEntity
		if attributes.GetVerb() == schema.AuthorizationActionImpersonate {
			resourceEntityFunc = ImpersonatedResourceToCedarEntity
		}
	}
	entity := resourceEntityFunc(attributes)
	req.Resource = entity.UID
	reqEntities[entity.UID] = &entity

	return reqEntities, req
}

func diagnosticToReason(diagnostic cedar.Diagnostic) string {
	if len(diagnostic.Reasons) == 0 {
		return ""
	}
	// TODO handle diagnostic.Errors?
	data, err := json.Marshal(diagnostic)
	if err != nil {
		return fmt.Sprintf("error marshalling diagnostic: %v", err)
	}
	return string(data)

}
