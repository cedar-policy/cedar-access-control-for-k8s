package convert

import (
	"testing"

	schema "github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
)

func TestParseSchemaName(t *testing.T) {
	testCases := []struct {
		name                                        string
		intput                                      string
		wantNs, wantAPIGroup, wantVersion, wantKind string
	}{
		{
			"DaemonSet",
			"io.k8s.api.apps.v1.DaemonSet",
			"",
			"apps",
			"v1",
			"DaemonSet",
		},
		{
			"ConfigMap",
			"io.k8s.api.core.v1.ConfigMap",
			"",
			"core",
			"v1",
			"ConfigMap",
		},
		{
			"Cedar Policy",
			"aws.k8s.cedar.v1.Policy",
			"aws::k8s",
			"cedar",
			"v1",
			"Policy",
		},
		{
			"too short",
			"aws.cedar.v1",
			"",
			"",
			"",
			"",
		},
		{
			"Object meta",
			"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			"",
			"meta",
			"v1",
			"ObjectMeta",
		},
		{
			"CRD",
			"io.cert-manager.v1.ClusterIssuer",
			"io",
			"cert_manager",
			"v1",
			"ClusterIssuer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotNs, gotAPIGroup, gotVersion, gotKind := ParseSchemaName(tc.intput)
			if gotNs != tc.wantNs {
				t.Fatalf("unexpected ns: got %q, want %q", gotNs, tc.wantNs)
			}
			if gotAPIGroup != tc.wantAPIGroup {
				t.Fatalf("unexpected apigroup: got %q, want %q", gotAPIGroup, tc.wantAPIGroup)
			}
			if gotVersion != tc.wantVersion {
				t.Fatalf("unexpected version: got %q, want %q", gotVersion, tc.wantVersion)
			}
			if gotKind != tc.wantKind {
				t.Fatalf("unexpected kind: got %q, want %q", gotKind, tc.wantKind)
			}
		})
	}
}

func TestSchemaNameToCedar(t *testing.T) {
	cases := []struct {
		name             string
		input            string
		wantNs, wantName string
	}{
		{
			"K8s auth api",
			"io.k8s.api.authentication.v1.TokenRequest",
			"authentication::v1",
			"TokenRequest",
		},
		{
			"K8s autoscaling API",
			"io.k8s.api.autoscaling.v1.Scale",
			"autoscaling::v1",
			"Scale",
		},
		{
			"core K8s api",
			"io.k8s.api.core.v1.ConfigMap",
			"core::v1",
			"ConfigMap",
		},
		{
			"coordination K8s api",
			"io.k8s.api.coordination.v1.Lease",
			"coordination::v1",
			"Lease",
		},
		{
			"k8s meta API",
			"io.k8s.apimachinery.pkg.apis.meta.v1.Status",
			"meta::v1",
			"Status",
		},
		// Should never get this case, we filter it out elsewhere
		// {
		// 	"k8s resoruce API",
		// 	"io.k8s.apimachinery.pkg.api.resource.Quantity",
		// 	"api::resource",
		// 	"Quantity",
		// },
		{
			"Cedar policy resource",
			"aws.k8s.cedar.v1.Policy",
			"aws::k8s::cedar::v1",
			"Policy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotNs, gotName := SchemaNameToCedar(tc.input)
			if gotNs != tc.wantNs {
				t.Fatalf("unexpected ns: got %q, want %q", gotNs, tc.wantNs)
			}
			if gotName != tc.wantName {
				t.Fatalf("unexpected name: got %q, want %q", gotName, tc.wantName)
			}
		})
	}
}

func TestRefToRelativeTypeName(t *testing.T) {
	cases := []struct {
		name, input, currentNs, want string
	}{
		{
			name:  "CRD",
			input: `#/components/schemas/aws.k8s.cedar.v1.Policy`,
			want:  "aws::k8s::cedar::v1::Policy",
		},
		{
			name:  "K8s API",
			input: `#/components/schemas/io.k8s.api.core.v1.ConfigMap`,
			want:  "core::v1::ConfigMap",
		},
		{
			name:  "Meta API",
			input: `#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta`,
			want:  "meta::v1::ObjectMeta",
		},
		{
			name:  "in-tree API group",
			input: `#/components/schemas/io.k8s.api.apps.v1.DaemonSet`,
			want:  "apps::v1::DaemonSet",
		},
		{
			name:      "in-tree API group, same namespace",
			input:     `#/components/schemas/io.k8s.api.core.v1.PodSpec`,
			currentNs: "#/components/schemas/io.k8s.api.core.v1.Pod",
			want:      "PodSpec",
		},
		{
			name:  "Time to string",
			input: `#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.Time`,
			want:  schema.StringType,
		},
		{
			name:  "MicroTime to string",
			input: `#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.MicroTime`,
			want:  schema.StringType,
		},
		{
			name:  "Quantity to string",
			input: `#/components/schemas/io.k8s.apimachinery.pkg.api.resource.Quantity`,
			want:  schema.StringType,
		},
		{
			name:  "RawExtension to String",
			input: `#/components/schemas/io.k8s.apimachinery.pkg.runtime.RawExtension`,
			want:  schema.StringType,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refToRelativeTypeName(tc.currentNs, tc.input)
			if got != tc.want {
				t.Fatalf("unexpected output: got %q, want %q", got, tc.want)
			}
		})
	}
}
