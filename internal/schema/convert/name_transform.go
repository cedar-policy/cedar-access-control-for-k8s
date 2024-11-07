package convert

import (
	"slices"
	"strings"

	schema "github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
)

func ParseSchemaName(schemaName string) (ns, apiGroup, version, kind string) {
	schemaName = strings.ReplaceAll(schemaName, "-", "_")
	strs := strings.Split(schemaName, ".")
	if len(strs) < 4 {
		return
	}
	slices.Reverse(strs)

	if strings.HasPrefix(schemaName, "io.k8s.api.") {
		strs = strs[:len(strs)-3]
	} else if strings.HasPrefix(schemaName, "io.k8s.apimachinery.pkg.apis.meta") {
		strs = strs[:len(strs)-4]
	} else {
		nsParts := strs[3:]
		slices.Reverse(nsParts)
		ns = strings.Join(nsParts, "::")
	}

	kind = strs[0]
	version = strs[1]
	apiGroup = strs[2]
	return
}

func SchemaNameToCedar(schemaName string) (ns, typeName string) {
	ns, apiGroup, version, kind := ParseSchemaName(schemaName)
	if ns != "" {
		return strings.Join([]string{ns, apiGroup, version}, "::"), kind
	}
	return strings.Join([]string{apiGroup, version}, "::"), kind
}

// Transform `"#/components/schemas/io.k8s.api.apps.v1.DaemonSetSpec"`
// into `apps::v1::DaemonSetSpec`
func refToRelativeTypeName(current, ref string) string {
	curParsed, found := strings.CutPrefix(current, "#/components/schemas/")
	if !found {
		curParsed = current
	}
	currentNs, _ := SchemaNameToCedar(curParsed)

	refParsed, found := strings.CutPrefix(ref, "#/components/schemas/")
	if !found {
		refParsed = ref
	}
	refNs, refType := SchemaNameToCedar(refParsed)

	if (refNs == "meta::v1" && refType == "Time") ||
		(refNs == "meta::v1" && refType == "MicroTime") ||
		(refNs == "io::k8s::apimachinery::pkg::util::intstr" && refType == "IntOrString") ||
		(refNs == "io::k8s::apimachinery::pkg::api::resource" && refType == "Quantity") ||
		(refNs == "io::k8s::apimachinery::pkg::runtime" && refType == "RawExtension") {
		return schema.StringType
	}

	if currentNs == refNs {
		return refType
	}
	return refNs + "::" + refType
}
