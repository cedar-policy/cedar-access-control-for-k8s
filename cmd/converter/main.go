package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cedarv1alpha1 "github.com/awslabs/cedar-access-control-for-k8s/api/v1alpha1"
	"github.com/awslabs/cedar-access-control-for-k8s/internal/convert"
	"github.com/cedar-policy/cedar-go"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

func main() {
	format := flag.String("output", "cedar", "Output format. One of [cedar, crd, json]")
	namespace := flag.String("namespace", "default", "Namespace to query when getting a single rolebinding")
	klog.InitFlags(flag.CommandLine)
	flag.Parse()
	defer klog.Flush()

	kind := ""
	switch flag.Arg(0) {
	case "clusterrolebinding", "clusterrolebindings", "crb":
		kind = "clusterrolebinding"
	case "rolebinding", "rolebindings", "rb":
		kind = "rolebinding"
	default:
		klog.Fatalf("Invalid type to convert, must be one of [clusterrolebinding, rolebinding] : %s", flag.Arg(0))
	}

	resourceNames := []string{}
	if len(flag.Args()) > 1 {
		resourceNames = strings.Split(flag.Arg(1), ",")
	}

	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %v", err)
	}
	ctx := context.Background()

	switch kind {
	case "rolebinding":
		var rbs *rbacv1.RoleBindingList = &rbacv1.RoleBindingList{
			Items: []rbacv1.RoleBinding{},
		}
		if len(resourceNames) == 0 {
			rbs, err = cs.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Fatalf("Error listing ClusterRoleBindings: %v", err)
			}

		} else {
			for _, resourceName := range resourceNames {
				rb, err := cs.RbacV1().RoleBindings(*namespace).Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Error getting RoleBinding %s: %v. Skipping this one", resourceName, err)
					continue
				}
				rbs.Items = append(rbs.Items, *rb)
			}
		}
		for i, binding := range rbs.Items {
			var ruler convert.Ruler
			switch binding.RoleRef.Kind {
			case "ClusterRole":
				cr, err := cs.RbacV1().ClusterRoles().Get(ctx, binding.RoleRef.Name, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Error getting ClusterRole %s: %v. Skipping this one", binding.RoleRef.Name, err)
					continue
				}
				ruler = convert.NewClusterRoleRuler(*cr)
			case "Role":
				role, err := cs.RbacV1().Roles(binding.Namespace).Get(ctx, binding.RoleRef.Name, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Error getting Role %s: %v. Skipping this one", binding.RoleRef.Name, err)
					continue
				}
				ruler = convert.NewRoleRuler(*role)
			}
			if i > 0 {
				fmt.Println()
				fmt.Println("// ----------------------------------------------------------------------------------------------------------------")
			}
			fmt.Println("// " + binding.Name)
			ps := convert.RoleBindingRulerToCedar(binding, ruler)
			switch *format {
			case "json":
				data, _ := ps.MarshalJSON()
				fmt.Println(string(data))
			case "cedar":
				fmt.Println(string(ps.MarshalCedar()))
			case "crd":
				crd := CRDForCedarPolicy(binding.Name, ps)
				data, err := yaml.Marshal(crd)
				if err != nil {
					klog.Fatalf("Error marshalling CRD: %v", err)
				}
				fmt.Println(string(data))
				if i != len(rbs.Items)-1 {
					fmt.Println("---")
				}
			default:
				klog.Fatalf("Invalid output format: %s", *format)
			}
		}

	case "clusterrolebinding":
		var crbs *rbacv1.ClusterRoleBindingList = &rbacv1.ClusterRoleBindingList{
			Items: []rbacv1.ClusterRoleBinding{},
		}
		if len(resourceNames) == 0 {
			crbs, err = cs.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Fatalf("Error listing ClusterRoleBindings: %v", err)
			}
		} else {
			for _, resourceName := range resourceNames {
				crb, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Error getting ClusterRoleBinding %s: %v. Skipping this one", resourceName, err)
					continue
				}
				crbs.Items = append(crbs.Items, *crb)
			}
		}
		for i, crb := range crbs.Items {
			cr, err := cs.RbacV1().ClusterRoles().Get(ctx, crb.RoleRef.Name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Error getting ClusterRole %s: %v. Skipping this one", crb.RoleRef.Name, err)
				continue
			}
			if i > 0 {
				fmt.Println()
				fmt.Println("// ----------------------------------------------------------------------------------------------------------------")
			}
			fmt.Println("// " + crb.Name)
			ps := convert.ClusterRoleBindingToCedar(crb, *cr)
			switch *format {
			case "json":
				data, _ := ps.MarshalJSON()
				fmt.Println(string(data))
			case "cedar":
				fmt.Println(string(ps.MarshalCedar()))
			case "crd":
				crd := CRDForCedarPolicy(crb.Name, ps)
				data, err := yaml.Marshal(crd)
				if err != nil {
					klog.Fatalf("Error marshalling CRD: %v", err)
				}
				fmt.Println(string(data))
				if i != len(crbs.Items)-1 {
					fmt.Println("---")
				}
			default:
				klog.Fatalf("Invalid output format: %s", *format)
			}
		}
	}
}

func CRDForCedarPolicy(name string, policies *cedar.PolicySet) *cedarv1alpha1.Policy {
	marshalled := policies.MarshalCedar()
	return &cedarv1alpha1.Policy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cedar.k8s.aws/v1alpha1",
			Kind:       "Policy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ReplaceAll(name, ":", "."),
		},
		Spec: cedarv1alpha1.PolicySpec{
			Content: string(marshalled),
		},
	}
}
