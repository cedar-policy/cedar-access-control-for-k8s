package schema

import (
	"slices"

	cedartypes "github.com/cedar-policy/cedar-go/types"
)

const (
	PrincipalUIDEntityName   = "PrincipalUID"
	NonResourceURLEntityName = "NonResourceURL"
	ResourceEntityName       = "Resource"
	FieldRequirementName     = "FieldRequirement"
	LabelRequirementName     = "LabelRequirement"

	AuthorizationActionEntityType = cedartypes.EntityType("k8s::Action")
	PrincipalUIDEntityType        = cedartypes.EntityType("k8s::" + PrincipalUIDEntityName)
	NonResourceURLEntityType      = cedartypes.EntityType("k8s::" + NonResourceURLEntityName)
	ResourceEntityType            = cedartypes.EntityType("k8s::" + ResourceEntityName)

	StringType = "String"
	LongType   = "Long"
	BoolType   = "Boolean"
	SetType    = "Set"
	RecordType = "Record"
	EntityType = "Entity"
)

// PrincipalUIDEntity returns a Cedar Entity for a PrincipalUID
func PrincipalUIDEntity() Entity {
	return Entity{
		MemberOfTypes: []string{},
		Shape: EntityShape{
			Type:       RecordType,
			Attributes: map[string]EntityAttribute{},
		},
	}
}

// NonResourceURLEntity returns a Cedar Entity for a NonResourceURL
func NonResourceURLEntity() Entity {
	return Entity{
		MemberOfTypes: []string{},
		Shape: EntityShape{
			Type: RecordType,
			Attributes: map[string]EntityAttribute{
				"path": {Type: StringType, Required: true},
			},
		},
	}
}

// FieldRequirementEntityShape returns a Cedar EntityShape for a FieldRequirement
func FieldRequirementEntityShape() EntityShape {
	return EntityShape{
		Type: RecordType,
		Attributes: map[string]EntityAttribute{
			"key":   {Type: StringType, Required: true},
			"op":    {Type: StringType, Required: true},
			"value": {Type: StringType, Required: true},
		},
	}
}

// LabelRequirementEntityShape returns a Cedar EntityShape for a LabelRequirement
func LabelRequirementEntityShape() EntityShape {
	return EntityShape{
		Type: RecordType,
		Attributes: map[string]EntityAttribute{
			"key":      {Type: StringType, Required: true},
			"operator": {Type: StringType, Required: true},
			"values":   {Type: SetType, Element: &EntityAttributeElement{Type: StringType}, Required: true},
		},
	}
}

// ResourceEntity returns a Cedar Entity for a Kubernetes Authorization Resource
func ResourceEntity() Entity {
	return Entity{
		Shape: EntityShape{
			Type: RecordType,
			Attributes: map[string]EntityAttribute{
				"apiGroup":    {Type: StringType, Required: true},
				"resource":    {Type: StringType, Required: true},
				"namespace":   {Type: StringType},
				"name":        {Type: StringType},
				"subresource": {Type: StringType},
				"fieldSelector": {
					Type:     SetType,
					Required: false,
					Element:  &EntityAttributeElement{Type: FieldRequirementName},
				},
				"labelSelector": {
					Type:     SetType,
					Required: false,
					Element:  &EntityAttributeElement{Type: LabelRequirementName},
				},
			},
		},
	}
}

const (
	AuthorizationActionGet              = "get"
	AuthorizationActionList             = "list"
	AuthorizationActionWatch            = "watch"
	AuthorizationActionCreate           = "create"
	AuthorizationActionUpdate           = "update"
	AuthorizationActionPatch            = "patch"
	AuthorizationActionDelete           = "delete"
	AuthorizationActionDeletecollection = "deletecollection"
	AuthorizationActionUse              = "use"
	AuthorizationActionBind             = "bind"
	AuthorizationActionImpersonate      = "impersonate"
	AuthorizationActionApprove          = "approve"
	AuthorizationActionSign             = "sign"
	AuthorizationActionEscalate         = "escalate"
	AuthorizationActionAttest           = "attest"
	AuthorizationActionPut              = "put"
	AuthorizationActionPost             = "post"
	AuthorizationActionHead             = "head"
	AuthorizationActionOptions          = "options"
	AuthorizationActionReadOnly         = "readOnly"
)

// AllAuthorizationActionNames returns a list of all Cedar Authorization Actions
func AllAuthorizationActionNames() []string {
	return []string{
		AuthorizationActionGet,
		AuthorizationActionList,
		AuthorizationActionWatch,
		AuthorizationActionCreate,
		AuthorizationActionUpdate,
		AuthorizationActionPatch,
		AuthorizationActionDelete,
		AuthorizationActionDeletecollection,
		AuthorizationActionUse,
		AuthorizationActionBind,
		AuthorizationActionImpersonate,
		AuthorizationActionApprove,
		AuthorizationActionSign,
		AuthorizationActionEscalate,
		AuthorizationActionAttest,
		AuthorizationActionPut,
		AuthorizationActionPost,
		AuthorizationActionHead,
		AuthorizationActionOptions,
		AuthorizationActionReadOnly,
	}
}

// GetAuthorizationActions returns a map of all Cedar Authorization Actions
func GetAuthorizationActions(principalNs, entityNs, actionNs string) map[string]ActionShape {
	readOnlyActions := []string{
		AuthorizationActionGet,
		AuthorizationActionList,
		AuthorizationActionWatch,
	}

	nonResourceOnlyActions := []string{
		AuthorizationActionPut,
		AuthorizationActionPost,
		AuthorizationActionHead,
		AuthorizationActionOptions,
	}

	resourceOnlyActions := []string{
		AuthorizationActionReadOnly,
		AuthorizationActionList,
		AuthorizationActionWatch,
		AuthorizationActionCreate,
		AuthorizationActionUpdate,
		AuthorizationActionDeletecollection,
		AuthorizationActionUse,
		AuthorizationActionBind,
		AuthorizationActionApprove,
		AuthorizationActionSign,
		AuthorizationActionEscalate,
		AuthorizationActionAttest,
	}

	principalPrefix := ""
	if principalNs != actionNs {
		principalPrefix = principalNs + "::"
	}
	entityPrefix := ""
	if entityNs != actionNs {
		entityPrefix = entityNs + "::"
	}

	// Set it to empty string if it's the same as the action namespace
	// so principal type names are not namespaced
	if principalNs == actionNs {
		principalNs = ""
	}

	response := map[string]ActionShape{}

	for _, action := range AllAuthorizationActionNames() {
		if action == AuthorizationActionImpersonate {
			continue
		}
		localActionShape := ActionShape{
			AppliesTo: ActionAppliesTo{
				PrincipalTypes: AuthorizationPrincipalTypes(principalNs),
				ResourceTypes: []string{
					entityPrefix + ResourceEntityName,
					entityPrefix + NonResourceURLEntityName,
				},
			},
		}
		if slices.Contains(readOnlyActions, action) {
			localActionShape.MemberOf = []ActionMember{{ID: AuthorizationActionReadOnly}}
		}
		if slices.Contains(nonResourceOnlyActions, action) {
			localActionShape.AppliesTo.ResourceTypes = []string{entityPrefix + NonResourceURLEntityName}
		}
		if slices.Contains(resourceOnlyActions, action) {
			localActionShape.AppliesTo.ResourceTypes = []string{entityPrefix + ResourceEntityName}
		}
		response[action] = localActionShape
	}
	// Manually add impersonate action
	response[AuthorizationActionImpersonate] = ActionShape{
		AppliesTo: ActionAppliesTo{
			PrincipalTypes: AuthorizationPrincipalTypes(principalNs),
			ResourceTypes: []string{
				principalPrefix + PrincipalUIDEntityName,
				principalPrefix + UserPrincipalType,
				principalPrefix + GroupPrincipalType,
				principalPrefix + ServiceAccountPrincipalType,
				principalPrefix + NodePrincipalType,
				principalPrefix + ExtraValuesType,
			},
		},
	}
	return response
}

func GetAuthorizationActionsNamespace(principalNs, entityNs, actionNs string) CedarSchemaNamespace {
	return CedarSchemaNamespace{
		Actions: GetAuthorizationActions(principalNs, entityNs, actionNs),
	}
}

func GetAuthorizationNamespace(principalNs, entityNs, actionNs string) CedarSchemaNamespace {
	return CedarSchemaNamespace{
		Actions: GetAuthorizationActions(principalNs, entityNs, actionNs),
		EntityTypes: map[string]Entity{
			PrincipalUIDEntityName:      PrincipalUIDEntity(),
			UserPrincipalType:           UserEntity(),
			GroupPrincipalType:          GroupEntity(),
			ServiceAccountPrincipalType: ServiceAccountEntity(),
			NodePrincipalType:           NodeEntity(),
			NonResourceURLEntityName:    NonResourceURLEntity(),
			ResourceEntityName:          ResourceEntity(),
			ExtraValuesType:             ExtraEntity(),
		},
		CommonTypes: map[string]EntityShape{
			FieldRequirementName:     FieldRequirementEntityShape(),
			LabelRequirementName:     LabelRequirementEntityShape(),
			ExtraValuesAttributeType: ExtraEntityShape(),
		},
	}
}
