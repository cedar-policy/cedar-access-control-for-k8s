package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	schema "github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type Path struct {
	ServerRelativeURL string `json:"serverRelativeURL"`
}

type PathDocument struct {
	Paths map[string]Path `json:"paths"`
}

type K8sSchemaGetter struct {
	client *rest.RESTClient
}

func NewK8sSchemaGetter(cfg *rest.Config) (*K8sSchemaGetter, error) {
	cfg.GroupVersion = &runtimeschema.GroupVersion{Group: "apidiscovery.k8s.io", Version: "v1"}
	cfg.APIPath = "/apis"
	cfg.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme).WithoutConversion()

	cli, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %v", err)
	}
	return &K8sSchemaGetter{client: cli}, nil
}

func (g *K8sSchemaGetter) GetAPISchema(suffix string) (*spec3.OpenAPI, error) {
	uri := path.Join("/openapi/v3", suffix)
	data, err := g.client.Get().AbsPath(uri).DoRaw(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get openapi: %v", err)
	}
	resp := &spec3.OpenAPI{}
	err = resp.UnmarshalJSON(data)
	return resp, err
}

func (g *K8sSchemaGetter) GetAllVersionedSchemas() ([]string, error) {
	data, err := g.client.Get().AbsPath("/openapi/v3").DoRaw(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get openapi: %v", err)
	}
	pathDoc := &PathDocument{}
	err = json.Unmarshal(data, pathDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal openapi: %v", err)
	}
	resp := []string{}

	matcher := regexp.MustCompile(`/v\d+(?:alpha\d+|beta\d+)?$`)
	for k := range pathDoc.Paths {
		// search for versioned APIs, ending in `/v*`
		if matcher.MatchString(k) {
			resp = append(resp, k)
		}
	}
	return resp, nil
}

func ModifySchemaForAPIVersion(openApiSchema *spec3.OpenAPI, cSchema schema.CedarSchema, api, version, actionNamespace string) error {

	for schemaKind, schemaDefinition := range openApiSchema.Components.Schemas {

		if strings.Contains(schemaKind, "io.k8s.kube-aggregator.pkg.apis") {
			continue
		}

		apiNs, apiGroup, sVersion, sKind := ParseSchemaName(schemaKind)

		if apiNs == "pkg.apimachinery.k8s.io" ||
			(apiGroup == "meta" && sVersion == "v1" && (sKind == "Time" || sKind == "MicroTime")) {
			continue
		}

		if sVersion != version {
			klog.V(5).Infof("Skipping %s version %q, not in version %q", schemaKind, sVersion, version)
			continue
		}

		nsName, _ := SchemaNameToCedar(schemaKind)
		klog.V(2).Infof("Processing %s", schemaKind)

		// ensure namespace exists in schema
		ns, ok := cSchema[nsName]
		if !ok {
			ns = schema.CedarSchemaNamespace{
				EntityTypes: map[string]schema.Entity{},
				Actions:     map[string]schema.ActionShape{},
				CommonTypes: map[string]schema.EntityShape{},
			}
			cSchema[nsName] = ns
		}
		// if the namespace doesn't contain that type, lets create it
		_, etOk := ns.EntityTypes[sKind]
		_, ctOk := ns.CommonTypes[sKind]
		if etOk || ctOk {
			continue
		}
		// K8s is only ever a string for Type
		if len(schemaDefinition.Type) == 0 {
			klog.V(5).Infof("Skipping unknown type %s", schemaKind)
			continue
		}
		var entity schema.Entity
		switch schemaDefinition.Type[0] {
		case "object":

			shape, err := RefToEntityShape(openApiSchema, schemaKind)
			if err != nil {
				klog.ErrorS(err, "Failed to serialize entity", "kind", schemaKind)
				continue
			}

			entity = schema.Entity{Shape: shape}

			// Handle empty objects like
			// `io.k8s.apimachinery.pkg.apis.meta.v1.FieldsV1`
			if schemaDefinition.Properties == nil {
				entity.Shape.Attributes = map[string]schema.EntityAttribute{}
			}
		case "string":
			entity = schema.Entity{
				Shape: schema.EntityShape{
					Type:       schema.StringType,
					Attributes: map[string]schema.EntityAttribute{},
				},
			}
		default:
			klog.V(5).Infof("Skipping unknown type %s on %s", schemaDefinition.Type[0], schemaKind)
		}

		if !isEntity(entity.Shape) {
			ns.CommonTypes[sKind] = entity.Shape
			continue
		}

		ns.EntityTypes[sKind] = entity
		// TODO: probably have to scan the schema for valid actions?
		schema.AddResourceTypeToAction(cSchema, actionNamespace, schema.AdmissionCreateAction, nsName+"::"+sKind)
		schema.AddResourceTypeToAction(cSchema, actionNamespace, schema.AdmissionDeleteAction, nsName+"::"+sKind)
		schema.AddResourceTypeToAction(cSchema, actionNamespace, schema.AdmissionUpdateAction, nsName+"::"+sKind)
		schema.AddResourceTypeToAction(cSchema, actionNamespace, schema.AdmissionConnectAction, nsName+"::"+sKind)
		schema.AddResourceTypeToAction(cSchema, actionNamespace, schema.AllAction, nsName+"::"+sKind)
	}

	return nil
}

// isEntity determines if the structure is an entity (true) or common type (false)
func isEntity(shape schema.EntityShape) bool {
	if shape.Attributes == nil {
		return false
	}
	if apiVersionAttr, ok := shape.Attributes["apiVersion"]; !ok || apiVersionAttr.Type != schema.StringType {
		return false
	}

	if kindAttr, ok := shape.Attributes["kind"]; !ok || kindAttr.Type != schema.StringType {
		return false
	}

	if metadataAttr, ok := shape.Attributes["metadata"]; !ok || (metadataAttr.Type != "meta::v1::ListMeta" && metadataAttr.Type != "meta::v1::ObjectMeta") {
		return false
	}
	return true
}

func getRequestBody(operation *spec3.Operation) string {
	if operation == nil {
		return ""
	}
	if operation.RequestBody == nil {
		return ""
	}
	if operation.RequestBody.Content == nil {
		return ""
	}
	ct, ok := operation.RequestBody.Content["*/*"]
	if !ok {
		return ""
	}
	if ct.Schema == nil {
		return ""
	}
	return ct.Schema.Ref.String()
}

func GetSchemasForAdmissionActions(api *spec3.OpenAPI) []string {
	resp := map[string]bool{}

	for _, path := range api.Paths.Paths {
		if path.Post != nil {
			reqBodyType := getRequestBody(path.Post)
			if reqBodyType != "" {

				parts := strings.Split(reqBodyType, "/")
				kindName := parts[len(parts)-1]
				resp[kindName] = true
			}
			// TODO: I think we're safe to ignore the response structures?
		}
		if path.Put != nil {
			reqBodyType := getRequestBody(path.Post)
			if reqBodyType != "" {
				parts := strings.Split(reqBodyType, "/")
				kindName := parts[len(parts)-1]
				resp[kindName] = true
			}
		}
		// if path.Delete != nil {
		// 	// TODO: Do we need to get response types? Shouldn't they always be covered in the put/post?
		// }
		// if path.Patch != nil {
		// 	// TODO: Do we need to get response types? Shouldn't they always be covered in the put/post?
		// }
	}
	keys := make([]string, 0, len(resp))
	for k := range resp {
		keys = append(keys, k)
	}
	return keys
}

func RefToEntityShape(api *spec3.OpenAPI, schemaKind string) (schema.EntityShape, error) {
	entityShape := schema.EntityShape{
		Type:       schema.RecordType,
		Attributes: map[string]schema.EntityAttribute{},
	}
	schemaDefinition, ok := api.Components.Schemas[schemaKind]
	if !ok {
		return entityShape, fmt.Errorf("schema %s not found", schemaKind)
	}

	for attrName, attrDef := range schemaDefinition.Properties {

		if len(attrDef.Type) != 0 {
			switch attrDef.Type[0] {
			case "string":
				entityShape.Attributes[attrName] = schema.EntityAttribute{
					Type:     schema.StringType,
					Required: slices.Contains(schemaDefinition.Required, attrName),
				}

			case "integer":
				entityShape.Attributes[attrName] = schema.EntityAttribute{
					Type:     schema.LongType,
					Required: slices.Contains(schemaDefinition.Required, attrName),
				}
			case "boolean":
				entityShape.Attributes[attrName] = schema.EntityAttribute{
					Type:     schema.BoolType,
					Required: slices.Contains(schemaDefinition.Required, attrName),
				}
			case "array":
				if attrDef.Items != nil && len(attrDef.Items.Schema.Type) > 0 {
					switch attrDef.Items.Schema.Type[0] {
					case "string":
						entityShape.Attributes[attrName] = schema.EntityAttribute{
							Type:     schema.SetType,
							Element:  &schema.EntityAttributeElement{Type: schema.StringType},
							Required: slices.Contains(schemaDefinition.Required, attrName),
						}
					case "integer":
						entityShape.Attributes[attrName] = schema.EntityAttribute{
							Type:     schema.SetType,
							Element:  &schema.EntityAttributeElement{Type: schema.LongType},
							Required: slices.Contains(schemaDefinition.Required, attrName),
						}
					case "boolean":
						entityShape.Attributes[attrName] = schema.EntityAttribute{
							Type:     schema.SetType,
							Element:  &schema.EntityAttributeElement{Type: schema.BoolType},
							Required: slices.Contains(schemaDefinition.Required, attrName),
						}
					default:
						// skipping "object" for now?
						klog.V(2).Infof("Skipping %s attr %s array of type %s, not implemented", schemaKind, attrName, attrDef.Items.Schema.Type[0])
					}
				} else if attrDef.Items != nil && len(attrDef.Items.Schema.AllOf) > 0 {

					typeName := refToRelativeTypeName(schemaKind, attrDef.Items.Schema.AllOf[0].Ref.String())
					attrShape, err := RefToEntityShape(api, attrDef.Items.Schema.AllOf[0].Ref.String()[21:])
					if err != nil {
						return entityShape, err
					}

					element := &schema.EntityAttributeElement{Type: typeName}
					if strings.HasSuffix(schemaKind, "."+typeName+"List") || isEntity(attrShape) {
						element.Type = schema.EntityType
						element.Name = typeName
					}

					entityShape.Attributes[attrName] = schema.EntityAttribute{
						Type:     schema.SetType,
						Element:  element,
						Required: slices.Contains(schemaDefinition.Required, attrName),
					}
				}
			case "object":
				if attrDef.Properties != nil {
					attrs, err := parseCRDProperties(15, attrDef.Properties)
					if err != nil {
						klog.ErrorS(err, "Failed to serialize entity", "kind", schemaKind)
						continue
					}
					entityShape.Attributes[attrName] = schema.EntityAttribute{
						Type:       schema.RecordType,
						Attributes: attrs,
						Required:   slices.Contains(schemaDefinition.Required, attrName),
					}
					continue
				}

				if attrDef.AdditionalProperties == nil {
					klog.V(5).Infof("Skipping %s attr %s object with no AdditionalProperties", schemaKind, attrName)
					continue
				}
				if attrDef.AdditionalProperties.Schema == nil {
					klog.V(5).Infof("Skipping %s attr %s object with no schema on AdditionalProperties", schemaKind, attrName)
					continue
				}

				if url := attrDef.AdditionalProperties.Schema.Ref.GetURL(); url != nil && url.String() != "" {
					typeName := refToRelativeTypeName(schemaKind, url.String())

					attrShape, err := RefToEntityShape(api, url.String()[21:])
					if err != nil {
						return entityShape, err
					}

					element := schema.EntityAttribute{
						Type:     typeName,
						Required: slices.Contains(schemaDefinition.Required, attrName),
					}
					if isEntity(attrShape) {
						element.Type = schema.EntityType
						element.Name = typeName
					}

					entityShape.Attributes[attrName] = element
					continue
				}

				// for string/string maps, hack to use custom KeyValue or KeyValueSlice types
				knownKeyValueStringMapAttributes := map[string][]string{
					"io.k8s.api.core.v1.ConfigMap":                       {"data", "binaryData"}, // format is []byte, should we exclude?
					"io.k8s.api.core.v1.CSIPersistentVolumeSource":       {"volumeAttributes"},
					"io.k8s.api.core.v1.CSIVolumeSource":                 {"volumeAttributes"},
					"io.k8s.api.core.v1.FlexPersistentVolumeSource":      {"options"},
					"io.k8s.api.core.v1.FlexVolumeSource":                {"options"},
					"io.k8s.api.core.v1.PersistentVolumeClaimStatus":     {"allocatedResourceStatuses"},
					"io.k8s.api.core.v1.PodSpec":                         {"nodeSelector"},
					"io.k8s.api.core.v1.ReplicationControllerSpec":       {"selector"},
					"io.k8s.api.core.v1.Secret":                          {"data", "stringData"},
					"io.k8s.api.core.v1.ServiceSpec":                     {"selector"},
					"io.k8s.api.discovery.v1.Endpoint":                   {"deprecatedTopology"},
					"io.k8s.api.node.v1.Scheduling":                      {"nodeSelector"},
					"io.k8s.api.storage.v1.StorageClass":                 {"parameters"},
					"io.k8s.api.storage.v1.VolumeAttachmentStatus":       {"attachmentMetadata"},
					"io.k8s.apimachinery.pkg.apis.meta.v1.LabelSelector": {"matchLabels"},
					"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta":    {"annotations", "labels"},
				}

				if attrs, ok := knownKeyValueStringMapAttributes[schemaKind]; ok &&
					slices.Contains(attrs, attrName) &&
					len(attrDef.AdditionalProperties.Schema.Type) > 0 && attrDef.AdditionalProperties.Schema.Type[0] == "string" {
					entityShape.Attributes[attrName] = schema.EntityAttribute{
						Type: schema.SetType,
						Element: &schema.EntityAttributeElement{
							Type: "meta::v1::KeyValue",
						},
					}
					continue
				}

				knownKeyValueStringStringSlice := map[string][]string{
					"io.k8s.api.authentication.v1.UserInfo":                    {"extra"},
					"io.k8s.api.authorization.v1.SubjectAccessReviewSpec":      {"extra"},
					"io.k8s.api.certificates.v1.CertificateSigningRequestSpec": {"extra"},
				}
				if attrs, ok := knownKeyValueStringStringSlice[schemaKind]; ok &&
					slices.Contains(attrs, attrName) &&
					len(attrDef.AdditionalProperties.Schema.Type) > 0 &&
					attrDef.AdditionalProperties.Schema.Type[0] == "array" &&
					attrDef.AdditionalProperties.Schema.Items != nil &&
					attrDef.AdditionalProperties.Schema.Items.Schema != nil &&
					len(attrDef.AdditionalProperties.Schema.Items.Schema.Type) > 0 &&
					attrDef.AdditionalProperties.Schema.Items.Schema.Type[0] == "string" {
					entityShape.Attributes[attrName] = schema.EntityAttribute{
						Type: schema.SetType,
						Element: &schema.EntityAttributeElement{
							Type: "meta::v1::KeyValueStringSlice",
						},
					}
					continue
				}

				klog.V(5).InfoS("Skipping object type", "kind", schemaKind, "attribute", attrName, "attrDef", attrDef)
				// skip until k/v objects are supported
				continue
			default:
				klog.V(5).Infof("Skipping %s attr %s type %s", schemaKind, attrName, attrDef.Type[0])
			}
		} else if len(attrDef.AllOf) != 0 {
			if len(attrDef.AllOf) != 1 {
				klog.V(5).Infof("Skipping %s attr %s that has more than one allOf", schemaKind, attrName)
				continue
			}

			typeName := refToRelativeTypeName(schemaKind, attrDef.AllOf[0].Ref.String())
			aea := schema.EntityAttribute{
				Type:     typeName,
				Required: slices.Contains(schemaDefinition.Required, attrName),
			}

			attrShape, err := RefToEntityShape(api, attrDef.AllOf[0].Ref.String()[21:])
			if err != nil {
				return entityShape, err
			}
			if isEntity(attrShape) {
				aea.Type = schema.EntityType
				aea.Name = typeName
			}
			entityShape.Attributes[attrName] = aea
		} else {
			// TODO type:
			// io.k8s.api.core.v1.NamespaceCondition.lastTransitionTime
			// io.k8s.api.networking.v1.IngressRule.http
			klog.V(5).Infof("Skipping %s attr %s that has no .type or .allOf", schemaKind, attrName)
		}

	}
	return entityShape, nil
}

func parseCRDProperties(depth int, properties map[string]spec.Schema) (map[string]schema.EntityAttribute, error) {
	if depth == 0 {
		return nil, fmt.Errorf("max depth reached")
	}

	attrMap := map[string]schema.EntityAttribute{}
	for k, v := range properties {
		if len(v.Type) == 0 {
			klog.V(2).InfoS("Skipping attr with no type", "attr", k, "schema", v)
			continue
		}
		// TODO: validate length
		switch v.Type[0] {
		case "string":
			attrMap[k] = schema.EntityAttribute{Type: schema.StringType, Required: slices.Contains(v.Required, k)}
		case "integer":
			attrMap[k] = schema.EntityAttribute{Type: schema.LongType, Required: slices.Contains(v.Required, k)}
		case "boolean":
			attrMap[k] = schema.EntityAttribute{Type: schema.BoolType, Required: slices.Contains(v.Required, k)}
		case "array":
			if v.Items != nil && len(v.Items.Schema.Type) > 0 {
				switch v.Items.Schema.Type[0] {
				case "string":
					attrMap[k] = schema.EntityAttribute{
						Type:     schema.SetType,
						Element:  &schema.EntityAttributeElement{Type: schema.StringType},
						Required: slices.Contains(v.Required, k),
					}
				case "integer":
					attrMap[k] = schema.EntityAttribute{
						Type:     schema.SetType,
						Element:  &schema.EntityAttributeElement{Type: schema.LongType},
						Required: slices.Contains(v.Required, k),
					}
				case "boolean":
					attrMap[k] = schema.EntityAttribute{
						Type:     schema.SetType,
						Element:  &schema.EntityAttributeElement{Type: schema.BoolType},
						Required: slices.Contains(v.Required, k),
					}
				default:
					// skipping "object" for now?
					klog.V(2).Infof("Skipping attr %s array of type %s, not implemented", k, v.Items.Schema.Type[0])
				}
			}
		case "object":
			// TODO: Better heuristic to tell if its a pod template
			if k == "podTemplate" {
				attrMap[k] = schema.EntityAttribute{
					Type:     "core::v1::PodTemplate",
					Required: slices.Contains(v.Required, k),
				}
				continue
			}

			// klog.V(2).Infof("Skipping %s attr %s object", k, v.Type[0])
			if v.Properties != nil {
				attrs, err := parseCRDProperties(depth-1, v.Properties)
				if err != nil {
					return nil, err
				}
				attrMap[k] = schema.EntityAttribute{Type: schema.RecordType, Attributes: attrs}
			}
		default:
			klog.V(2).Infof("Skipping attr %s type %s", k, v.Type[0])
		}
	}
	return attrMap, nil

}
