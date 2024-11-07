package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	"k8s.io/kube-openapi/pkg/spec3"
)

func createSchema() schema.CedarSchema {
	cedarschema := schema.NewCedarSchema()
	authorizationNs := "k8s"
	admissionActionNs := "k8s::admission"
	cedarschema[authorizationNs] = schema.GetAuthorizationNamespace(authorizationNs, authorizationNs, authorizationNs)
	schema.AddAdmissionActions(cedarschema, admissionActionNs, authorizationNs)
	return cedarschema
}

func TestModifySchemaForAPIVersion(t *testing.T) {
	cases := []struct {
		name   string
		inputs []struct {
			inputOpenAPIFile        string
			inputName, inputVersion string
		}
		wantFunc func(t *testing.T, got schema.CedarSchema)
	}{
		{
			name: "RBAC API",
			inputs: []struct {
				inputOpenAPIFile        string
				inputName, inputVersion string
			}{
				{
					inputOpenAPIFile: "apis.rbac.authorization.k8s.io.v1.schema.json",
					inputName:        "rbac",
					inputVersion:     "v1",
				},
			},
			wantFunc: func(t *testing.T, got schema.CedarSchema) {
				if _, ok := got["rbac::v1"].CommonTypes["ClusterRole"]; ok {
					t.Fatalf("ClusterRole should be an entity not a common type")
				}
			},
		},
		{
			name: "Apps API",
			inputs: []struct {
				inputOpenAPIFile        string
				inputName, inputVersion string
			}{
				{
					inputOpenAPIFile: "apis.apps.v1.schema.json",
					inputName:        "apps",
					inputVersion:     "v1",
				},
				{
					inputOpenAPIFile: "api.v1.schema.json",
					inputName:        "core",
					inputVersion:     "v1",
				},
			},
			wantFunc: func(t *testing.T, got schema.CedarSchema) {
				// StatefulSet.spec.volumeClaimTemplates are the one in-tree API that has an entity as an attribute
				// Validate that we set it correctly
				if got["apps::v1"].CommonTypes["StatefulSetSpec"].Attributes["volumeClaimTemplates"].Element.Type != schema.EntityType {
					t.Fatalf("volumeClaimTemplates should be an entity not a common type")
				}

				if _, ok := got["apps::v1"].CommonTypes["StatefulSet"]; ok {
					t.Fatalf("StatefulSet should be an entity not a common type")
				}
			},
		},
		{
			name: "Authentication API",
			inputs: []struct {
				inputOpenAPIFile        string
				inputName, inputVersion string
			}{
				{
					inputOpenAPIFile: "apis.authentication.k8s.io.v1.schema.json",
					inputName:        "authentication",
					inputVersion:     "v1",
				},
			},
			wantFunc: func(t *testing.T, got schema.CedarSchema) {
				if _, ok := got["authentication::v1"].CommonTypes["TokenRequest"]; ok {
					t.Fatalf("StatefulSet should be an entity not a common type")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cedarschema := createSchema()

			for _, input := range tc.inputs {
				data, err := os.ReadFile(filepath.Join("testdata", input.inputOpenAPIFile))
				if err != nil {
					t.Fatalf("error reading file %s: %v", input.inputOpenAPIFile, err)
				}
				api := &spec3.OpenAPI{}
				err = api.UnmarshalJSON(data)
				if err != nil {
					t.Fatalf("error unmarshalling file %s: %v", input.inputOpenAPIFile, err)
				}
				err = ModifySchemaForAPIVersion(api, cedarschema, input.inputName, input.inputVersion, "k8s::admission")
				if err != nil {
					t.Fatalf("error modifying schema for %s: %v", input.inputOpenAPIFile, err)
				}
			}
			tc.wantFunc(t, cedarschema)
		})
	}
}
