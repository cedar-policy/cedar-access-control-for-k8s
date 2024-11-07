package schema

import (
	"encoding/json"
	"slices"
	"strings"
)

// NewCedarSchema creates a new Cedar schema
func NewCedarSchema() CedarSchema {
	return map[string]CedarSchemaNamespace{}
}

// CedarSchema is the top level schema structure
type CedarSchema map[string]CedarSchemaNamespace

func (cs *CedarSchema) SortActionEntities() {
	for _, ns := range *cs {
		if ns.Actions != nil {
			for _, action := range ns.Actions {
				slices.Sort(action.AppliesTo.PrincipalTypes)
				slices.Sort(action.AppliesTo.ResourceTypes)
			}
		}
	}
}

// GetEntityShape returns the shape of an entity in the schema by its namespaced name
func (cs CedarSchema) GetEntityShape(name string) (*EntityShape, bool) {
	if cs == nil {
		return nil, false
	}
	parts := strings.Split(name, "::")
	namespaceName := ""
	if len(parts) > 1 {
		namespaceName = strings.Join(parts[:len(parts)-1], "::")
		name = parts[len(parts)-1]
	}
	ns, ok := cs[namespaceName]
	if !ok {
		return nil, false
	}
	if ns.EntityTypes == nil && ns.CommonTypes == nil {
		return nil, false
	}
	if ns.EntityTypes != nil {
		entity, ok := ns.EntityTypes[name]
		if ok {
			return &entity.Shape, true
		}
	}
	if ns.CommonTypes == nil {
		return nil, false
	}
	entityShape, ok := ns.CommonTypes[name]
	if ok {
		return &entityShape, true
	}
	return nil, false
}

// CedarSchemaNamespace represents a namespace within a schema
type CedarSchemaNamespace struct {
	EntityTypes map[string]Entity      `json:"entityTypes"`
	Actions     map[string]ActionShape `json:"actions"`
	CommonTypes map[string]EntityShape `json:"commonTypes,omitempty"`
}

// Entity represents a Cedar entity that defines principals and resources
type Entity struct {
	Shape         EntityShape `json:"shape"`
	MemberOfTypes []string    `json:"memberOfTypes,omitempty"`
}

// EntityShape represents the shape of a Cedar entity
type EntityShape struct {
	Type       string                     `json:"type"`
	Attributes map[string]EntityAttribute `json:"attributes"`
}

// EntityAttribute represents an attribute of a Cedar entity
//
// Element may on be used when the Type is "Set"
type EntityAttribute struct {
	Type       string
	Name       string
	Required   bool
	Element    *EntityAttributeElement
	Attributes map[string]EntityAttribute
}

// this is a gross hack to work around the fact that cedar assumes that the attributes field is always present if the type is
// "Record"
type recordEntityAttribute struct {
	Name       string                     `json:"name,omitempty"`
	Type       string                     `json:"type"`
	Required   bool                       `json:"required"` // omitempty is not used because cedar assumes its required
	Element    *EntityAttributeElement    `json:"element,omitempty"`
	Attributes map[string]EntityAttribute `json:"attributes"`
}

type nonRecordEntityAttribute struct {
	Name       string                     `json:"name,omitempty"`
	Type       string                     `json:"type"`
	Required   bool                       `json:"required"` // omitempty is not used because cedar assumes its required
	Element    *EntityAttributeElement    `json:"element,omitempty"`
	Attributes map[string]EntityAttribute `json:"attributes,omitempty"`
}

func (ea *EntityAttribute) toRecordEA() *recordEntityAttribute {
	return &recordEntityAttribute{
		Type:       ea.Type,
		Required:   ea.Required,
		Element:    ea.Element,
		Attributes: ea.Attributes,
		Name:       ea.Name,
	}
}

func (ea *EntityAttribute) toNonRecordEA() *nonRecordEntityAttribute {
	return &nonRecordEntityAttribute{
		Type:       ea.Type,
		Required:   ea.Required,
		Element:    ea.Element,
		Attributes: ea.Attributes,
		Name:       ea.Name,
	}
}

// MarshalJSON marshals an EntityAttribute to JSON
//
// the attributes are populated with an empty map if the Type is a "Record"
func (ea EntityAttribute) MarshalJSON() ([]byte, error) {
	if ea.Type == RecordType && len(ea.Attributes) == 0 {
		return json.Marshal(ea.toRecordEA())
	}
	return json.Marshal(ea.toNonRecordEA())
}

// EntityAttributeElement represents an element of a Cedar entity attribute
type EntityAttributeElement struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// ActionShape represents the shape of a Cedar action
type ActionShape struct {
	AppliesTo ActionAppliesTo `json:"appliesTo"`
	MemberOf  []ActionMember  `json:"memberOf,omitempty"`
}

// ActionMember represents a parent type of a Cedar action
type ActionMember struct {
	ID string `json:"id"`
}

// ActionAppliesTo contains the entity types that a Cedar action applies to
type ActionAppliesTo struct {
	PrincipalTypes []string `json:"principalTypes"`
	ResourceTypes  []string `json:"resourceTypes"`
}
