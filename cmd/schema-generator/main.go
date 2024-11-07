package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema/convert"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	authorizationNs := flag.String("authorization-namespace", "k8s", "Namespace for authorization entities and actions")
	// principalNs := flag.String("principal-namespace", "k8s", "Namespace for principal entities")
	actionNs := flag.String("admission-action-namespace", "k8s::admission", "Namespace for admission entities")
	addAdmissionTypes := flag.Bool("admission", true, "Add admission entities")
	sourceSchema := flag.String("source-schema", "", "File to read schema from ")
	outputFile := flag.String("output", "", "File to write schema to")

	klog.InitFlags(nil)
	flag.Parse()

	cedarschema := schema.NewCedarSchema()
	if *sourceSchema != "" {
		data, err := os.ReadFile(*sourceSchema)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		err = json.Unmarshal(data, &cedarschema)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
	}

	// add authorization namespace
	cedarschema[*authorizationNs] = schema.GetAuthorizationNamespace(*authorizationNs, *authorizationNs, *authorizationNs)

	if *addAdmissionTypes {
		if *actionNs == *authorizationNs {
			fmt.Printf("Admission and authorization namespaces cannot be the same\n")
			os.Exit(1)
		}

		// add actions
		schema.AddAdmissionActions(cedarschema, *actionNs, *authorizationNs)
		if actionNamespace, ok := cedarschema[*actionNs]; ok {
			actionNamespace.EntityTypes = make(map[string]schema.Entity)
			cedarschema[*actionNs] = actionNamespace
		} else {
			cedarschema[*actionNs] = schema.CedarSchemaNamespace{
				EntityTypes: make(map[string]schema.Entity),
				Actions:     make(map[string]schema.ActionShape),
				CommonTypes: make(map[string]schema.EntityShape),
			}
		}

		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")

		// Load the kubeconfig file to get the configuration
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			klog.ErrorS(err, "failed to build config from kubeconfig")
			os.Exit(1)
		}

		getter, err := convert.NewK8sSchemaGetter(cfg)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}

		apiGroups, err := getter.GetAllVersionedSchemas()
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		slices.Sort(apiGroups)

		// prefix to api/version
		apiNameVersionMap := map[string]struct {
			Name    string
			Version string
		}{}
		for _, g := range apiGroups {
			if g == "api/v1" {
				apiNameVersionMap[g] = struct {
					Name    string
					Version string
				}{
					Name:    "core",
					Version: "v1",
				}
				continue
			}
			parts := strings.Split(g, "/")

			apiNameVersionMap[g] = struct {
				Name    string
				Version string
			}{
				Name:    parts[1],
				Version: parts[2],
			}
		}
		// want to loop alphabetically by api group
		for _, k := range apiGroups {
			v := apiNameVersionMap[k]
			if v.Name == "apiextensions.k8s.io" {
				continue
			}
			klog.InfoS("Fetching schema for API", "api", v.Name, "version", v.Version)
			openAPISpec, err := getter.GetAPISchema(k)
			if err != nil {
				klog.ErrorS(err, "Failed to get schema for API, skipping", "api", v.Name, "version", v.Version)
				continue
			}
			klog.InfoS("Converting schema for API", "api", v.Name, "version", v.Version)

			// TODO: In order to find which Admission verbs apply to which resources:
			// * Get the APIResourceList{} (/{k}) for a given API
			// * In ModifySchemaForAPIVersion(),
			//    * Find the APIResourceList.resources[].Kind
			//    * Look for the corresponding admission verbs (delete/deletecollection/create/patch/update)
			//    * Figure out how to determine which subresource maps to CONNECT

			err = convert.ModifySchemaForAPIVersion(openAPISpec, cedarschema, v.Name, v.Version, *actionNs)
			if err != nil {
				klog.ErrorS(err, "Failed to get convert to cedar schema for API, skipping", "api", v.Name, "version", v.Version)
				continue
			}
		}
	}
	cedarschema.SortActionEntities()
	// TODO: ENTITY TAGS: this is just here until we get real key/value map support
	schema.ModifyObjectMetaMaps(cedarschema)

	data, err := json.MarshalIndent(cedarschema, "", "\t")
	if err != nil {
		klog.Fatalf("Failed to marshal schema: %v", err)
	}
	if *outputFile != "" {
		err = os.WriteFile(*outputFile, data, 0644)
		if err != nil {
			klog.Fatalf("Failed to write schema to file: %v", err)
		}
		return
	}
	fmt.Println(string(data))
}
