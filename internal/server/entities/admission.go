package entities

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	cedartypes "github.com/cedar-policy/cedar-go/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	AdmissionActionIDConnect cedartypes.String = `k8s::admission::Action::"connect"`
	AdmissionActionIDCreate  cedartypes.String = `k8s::admission::Action::"create"`
	AdmissionActionIDUpdate  cedartypes.String = `k8s::admission::Action::"update"`
	AdmissionActionIDDelete  cedartypes.String = `k8s::admission::Action::"delete"`
	AdmissionActionIDAll     cedartypes.String = `k8s::admission::Action::"all"`
)

var (
	runtimeScheme = k8sruntime.NewScheme()
)

func init() {
	// Adds all in-tree types from client-go to scheme so we can decode them
	utilruntime.Must(clientgoscheme.AddToScheme(runtimeScheme))
}

func AdmissionActionEntities() []*cedartypes.Entity {
	resp := []*cedartypes.Entity{{UID: cedartypes.EntityUID{Type: schema.AdmissionActionEntityType, ID: AdmissionActionIDAll}}}
	for _, actionID := range []cedartypes.String{
		AdmissionActionIDConnect,
		AdmissionActionIDCreate,
		AdmissionActionIDUpdate,
		AdmissionActionIDDelete} {
		resp = append(resp, &cedartypes.Entity{
			UID:     cedartypes.EntityUID{Type: schema.AdmissionActionEntityType, ID: actionID},
			Parents: []cedartypes.EntityUID{resp[0].UID},
		})
	}
	return resp
}

// get the principal UID, and all principal entities from the request
func CedarPrincipalEntitesFromAdmissionRequest(req *admission.Request) (*cedartypes.EntityUID, cedartypes.Entities, error) {
	if req == nil {
		return nil, nil, errors.New("request is nil")
	}
	principalUid, entities := UserToCedarEntity(&UserInfoWrapper{req.UserInfo})
	return &principalUid, entities, nil
}

func CedarActionEntityFromAdmissionRequest(req *admission.Request) (cedartypes.EntityUID, error) {
	resp := cedartypes.EntityUID{Type: schema.AdmissionActionEntityType}
	switch req.Operation {
	case "CONNECT":
		resp.ID = schema.AdmissionConnectAction
	case "CREATE":
		resp.ID = schema.AdmissionCreateAction
	case "UPDATE":
		resp.ID = schema.AdmissionUpdateAction
	case "DELETE":
		resp.ID = schema.AdmissionDeleteAction
	default:
		return resp, fmt.Errorf("unsupported operation %s", req.Operation)
	}
	return resp, nil
}

func AdmissionRequestToAuthorizerAttribute(req admission.Request) authorizer.Attributes {
	return &authorizerAttributeWrapper{req}
}

type authorizerAttributeWrapper struct {
	admission.Request
}

var _ authorizer.Attributes = &authorizerAttributeWrapper{}

func (a *authorizerAttributeWrapper) GetResource() string                            { return a.Resource.Resource }
func (a *authorizerAttributeWrapper) GetSubresource() string                         { return a.SubResource }
func (a *authorizerAttributeWrapper) GetAPIGroup() string                            { return a.Resource.Group }
func (a *authorizerAttributeWrapper) GetAPIVersion() string                          { return a.Resource.Version }
func (a *authorizerAttributeWrapper) GetNamespace() string                           { return a.Namespace }
func (a *authorizerAttributeWrapper) GetName() string                                { return a.Name }
func (a *authorizerAttributeWrapper) GetVerb() string                                { return string(a.Operation) }
func (a *authorizerAttributeWrapper) GetUser() user.Info                             { return &UserInfoWrapper{a.UserInfo} }
func (a *authorizerAttributeWrapper) IsResourceRequest() bool                        { return true }
func (a *authorizerAttributeWrapper) IsReadOnly() bool                               { return false }
func (a *authorizerAttributeWrapper) GetPath() string                                { return "" }
func (a *authorizerAttributeWrapper) GetFieldSelector() (fields.Requirements, error) { return nil, nil }
func (a *authorizerAttributeWrapper) GetLabelSelector() (labels.Requirements, error) { return nil, nil }

// type cedarEntityValueWrapper struct {
// 	cedartypes.Entity
// }

// var _ cedartypes.Value = &cedarEntityValueWrapper{}

// func (e cedarEntityValueWrapper) Equal(v cedartypes.Value) bool {
// 	return false
// }

// func (e cedarEntityValueWrapper) ExplicitMarshalJSON() ([]byte, error) {
// 	return json.Marshal(e.Entity)
// }

// func (e cedarEntityValueWrapper) MarshalCedar() []byte {
// 	return []byte{}
// }

// func (e cedarEntityValueWrapper) String() string {
// 	return ""
// }

// func (e cedarEntityValueWrapper) hash() uint64 {
// 	return 0
// }

func UnstructuredFromAdmissionRequestObject(data []byte) (*unstructured.Unstructured, error) {
	if data == nil {
		return nil, errors.New("unstructured data is nil")
	}
	obj := &unstructured.Unstructured{}

	err := obj.UnmarshalJSON(data)
	if err != nil {
		return nil, fmt.Errorf("error decoding generator resource %w", err)
	}
	return obj, nil
}

func CedarResourceEntityFromAdmissionRequest(req *admission.Request) (*cedartypes.Entity, error) {
	return cedarResourceEntityFromAdmissionRequest(req, req.Object.Raw)
}

func CedarOldResourceEntityFromAdmissionRequest(req *admission.Request) (*cedartypes.Entity, error) {
	return cedarResourceEntityFromAdmissionRequest(req, req.OldObject.Raw)
}

func cedarResourceEntityFromAdmissionRequest(req *admission.Request, rawData []byte) (*cedartypes.Entity, error) {
	// Convert the request's generator resource to unstructured for expansion
	obj, err := UnstructuredFromAdmissionRequestObject(rawData)
	if err != nil {
		return nil, fmt.Errorf("error getting unstructured resource %s: %w", req.Name, err)
	}

	// TODO:
	// * Generate the schema
	// * Update the schema on CRD changes
	// * Walk the schema for this type and build the Cedar entity

	resourceGroup := req.Resource.Group
	if resourceGroup == "" {
		resourceGroup = "core"
	}

	attributes, err := UnstructuredToRecord(obj, resourceGroup, req.Kind.Version, req.Kind.Kind)
	if err != nil {
		return nil, fmt.Errorf("error converting unstructured object to Cedar entity: %w", err)
	}

	cedarResourceType := strings.Join([]string{resourceGroup, req.Kind.Version, req.Kind.Kind}, "::")

	resp := cedartypes.Entity{
		UID: cedartypes.EntityUID{
			Type: cedartypes.EntityType(cedarResourceType),
			ID: cedartypes.String(
				ResourceRequestToPath(AdmissionRequestToAuthorizerAttribute(*req)),
			),
		},
		Attributes: attributes,
	}

	return &resp, nil
}

func UnstructuredToRecord(obj *unstructured.Unstructured, group, version, kind string) (cedartypes.Record, error) {
	if obj == nil {
		return cedartypes.NewRecord(nil), errors.New("unstructured object is nil")
	}
	attributes := map[cedartypes.String]cedartypes.Value{}
	for k, v := range obj.Object {
		if v == nil {
			// skip empty values
			continue
		}
		// Try not to blow the stack, limit CRDs to 32 fields deep
		val, err := walkObject(32, group, version, kind, k, v)
		if err != nil {
			return cedartypes.NewRecord(nil), err
		}
		if val == nil {
			// skip empty return values, such as empty objects
			continue
		}
		attributes[cedartypes.String(k)] = val
	}
	return cedartypes.NewRecord(cedartypes.RecordMap(attributes)), nil
}

func walkObject(i int, group, version, kind, keyName string, obj any) (cedartypes.Value, error) {
	if i == 0 {
		return nil, errors.New("max depth reached")
	}
	if obj == nil {
		// skip empty values
		return nil, nil
	}

	// TODO: ENTITY TAGS: This is a hack until key/value objects are supported
	// g/v/k/attrNames
	knownKeyValueStringMapAttributes := map[string]map[string]map[string][]string{
		"core": {
			"v1": {
				"ConfigMap":                   {"data", "binaryData"},
				"CSIPersistentVolumeSource":   {"volumeAttributes"},
				"CSIVolumeSource":             {"volumeAttributes"},
				"FlexPersistentVolumeSource":  {"options"},
				"FlexVolumeSource":            {"options"},
				"PersistentVolumeClaimStatus": {"allocatedResourceStatuses"},
				"Pod":                         {"nodeSelector"},
				"ReplicationController":       {"selector"},
				"Secret":                      {"data", "stringData"},
				"Service":                     {"selector"},
				// The following types only get evaulated if we walk the schema tree
				// "PodSpec":                     {"nodeSelector"},
				// "ReplicationControllerSpec":   {"selector"},
				// "ServiceSpec":                 {"selector"},
			},
		},
		"discovery": {"v1": {"Endpoint": {"deprecatedTopology"}}},
		"node":      {"v1": {"Scheduling": {"nodeSelectors"}}},
		"storage": {
			"v1": {
				"StorageClass":           {"parameters"},
				"VolumeAttachmentStatus": {"attachmentMetadata"},
			},
		},
		// TODO: the following only works once we walk the schema tree
		"meta": {
			"v1": {
				"LabelSelector": {"matchLabels"},
				"ObjectMeta":    {"annotations", "labels"},
			},
		},
	}
	if apiGroup, ok := knownKeyValueStringMapAttributes[group]; ok {
		if apiVersion, ok := apiGroup[version]; ok {
			if attrNames, ok := apiVersion[kind]; ok {
				if slices.Contains(attrNames, keyName) {
					klog.V(6).InfoS("Converting k/v pairs to meta::v1::KeyValue", "group", group, "version", version, "kind", kind, "attr", keyName, "value", obj)
					set := []cedartypes.Value{}
					for kk, vv := range obj.(map[string]interface{}) {

						val, ok := vv.(string)
						if !ok {
							klog.ErrorS(nil, "Error converting label/annotation value to string", "key", kk, "value", vv)
							break
						}
						set = append(set, cedartypes.NewRecord(cedartypes.RecordMap{
							cedartypes.String("key"):   cedartypes.String(kk),
							cedartypes.String("value"): cedartypes.String(val),
						}))
					}
					return cedartypes.NewSet(set), nil
				}
			}
		}
	}

	knownKeyValueStringSliceMapAttributes := map[string]map[string]map[string][]string{
		"authentication": {
			"v1": {
				"UserInfo": {"extra"},
			},
		},
		"authorization": {
			"v1": {
				"SubjectAccessReview": {"extra"},
			},
		},
		"certificates": {
			"v1": {
				"CertificateSigningRequest": {"extra"},
			},
		},
	}
	if apiGroup, ok := knownKeyValueStringSliceMapAttributes[group]; ok {
		if apiVersion, ok := apiGroup[version]; ok {
			if attrNames, ok := apiVersion[kind]; ok {
				if slices.Contains(attrNames, keyName) {
					klog.V(6).InfoS("Converting k/v pairs to meta::v1::KeyValueStringSlice", "group", group, "version", version, "kind", kind, "attr", keyName, "value", obj)
					set := []cedartypes.Value{}
					for kk, vv := range obj.(map[string]interface{}) {

						val, ok := vv.([]string)
						if !ok {
							klog.ErrorS(nil, "Error converting label/annotation value to slice of string", "key", kk, "value", vv)
							break
						}
						valSet := []cedartypes.Value{}
						for _, v := range val {
							valSet = append(valSet, cedartypes.String(v))
						}
						set = append(set, cedartypes.NewRecord(cedartypes.RecordMap{
							cedartypes.String("key"):   cedartypes.String(kk),
							cedartypes.String("value"): cedartypes.NewSet(valSet),
						}))
					}
					return cedartypes.NewSet(set), nil
				}
			}
		}
	}

	if _, ok := obj.(map[string]interface{}); (keyName == "labels" || keyName == "annotations") && ok {
		klog.V(6).InfoS("Converting labels/annotations to set of Cedar records", keyName, obj)
		set := []cedartypes.Value{}
		for kk, vv := range obj.(map[string]interface{}) {

			val, ok := vv.(string)
			if !ok {
				klog.ErrorS(nil, "Error converting label/annotation value to string", "key", kk, "value", vv)
				break
			}
			set = append(set, cedartypes.NewRecord(cedartypes.RecordMap{
				cedartypes.String("key"):   cedartypes.String(kk),
				cedartypes.String("value"): cedartypes.String(val),
			}))
		}
		return cedartypes.NewSet(set), nil
	}
	// End gross hack for key/value maps

	switch t := obj.(type) {
	case map[string]interface{}:
		rec := cedartypes.RecordMap{}
		for kk, vv := range obj.(map[string]interface{}) {
			val, err := walkObject(i-1, group, version, kind, kk, vv)
			if err != nil {
				return nil, err
			}
			if val == nil {
				// skip empty values
				continue
			}
			rec[cedartypes.String(kk)] = val
		}
		if len(rec) == 0 {
			// skip empty records
			return nil, nil
		}
		return cedartypes.NewRecord(rec), nil
	case []interface{}:
		set := []cedartypes.Value{}
		for _, item := range obj.([]interface{}) {
			val, err := walkObject(i-1, group, version, kind, keyName, item)
			if err != nil {
				return nil, err
			}
			set = append(set, val)
		}
		return cedartypes.NewSet(set), nil
	case string:
		// Try to parse the string as an IP address for
		// known IP address keys
		if slices.Contains([]string{"podIP", "clusterIP", "loadBalancerIP", "hostIP", "ip", "podIPs", "hostIPs"}, keyName) {
			addr, err := cedartypes.ParseIPAddr(obj.(string))
			if err != nil {
				return cedartypes.String(obj.(string)), nil
			}
			return addr, nil
		}
		return cedartypes.String(obj.(string)), nil
	case int:
		return cedartypes.Long(obj.(int)), nil
	case int64:
		return cedartypes.Long(obj.(int64)), nil
	case uint:
		return cedartypes.Long(obj.(uint)), nil
	case uint64:
		return cedartypes.Long(obj.(uint64)), nil
	case bool:
		return cedartypes.Boolean(obj.(bool)), nil
	default:
		return nil, fmt.Errorf("unsupported type %T", t)
	}

}
