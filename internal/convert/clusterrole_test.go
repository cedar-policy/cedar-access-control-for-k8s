package convert

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var update = flag.Bool("update", false, "update testdata")

func TestClusterRoleBindingToCedar(t *testing.T) {
	// TODO: Test case cleanup
	// * read testcases from input files containing CRB and CR objects
	// * add all of Kind's in-cluster crbs/crs
	// * migrate custom policies defined below to input files

	testCases := []struct {
		name string
		crb  rbacv1.ClusterRoleBinding
		cr   rbacv1.ClusterRole
	}{
		{
			name: "system:public-info-viewer",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:public-info-viewer",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "system:public-info-viewer",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "Group",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "system:authenticated",
					},
					{
						Kind:     "Group",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "system:unauthenticated",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:public-info-viewer",
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs: []string{"get"},
						NonResourceURLs: []string{
							"/healthz",
							"/livez",
							"/readyz",
							"/version",
							"/version/",
						},
					},
				},
			},
		},
		{
			name: "kubeadm:get-nodes",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeadm:get-nodes",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "kubeadm:get-nodes",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "Group",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "system:bootstrappers:kubeadm:default-node-token",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:public-info-viewer",
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"nodes"},
					},
				},
			},
		},
		{
			name: "system:coredns",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:coredns",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "system:coredns",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "coredns",
						Namespace: "kube-system",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:coredns",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Verbs:     []string{"list", "watch"},
						Resources: []string{"endpoints", "services", "pods", "namespaces"},
					},
					{
						APIGroups: []string{"discovery.k8s.io"},
						Verbs:     []string{"list", "watch"},
						Resources: []string{"endpointslices"},
					},
				},
			},
		},
		{
			name: "system:node-proxier",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:node-proxier",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "system:node-proxier",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "User",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "system:kube-proxy",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:node-proxier",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Verbs:     []string{"list", "watch"},
						Resources: []string{"endpoints", "services"},
					},
					{
						APIGroups: []string{""},
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{"nodes"},
					},
					{
						APIGroups: []string{"", "events.k8s.io"},
						Verbs:     []string{"create", "patch", "update"},
						Resources: []string{"events"},
					},
					{
						APIGroups: []string{"discovery.k8s.io"},
						Verbs:     []string{"list", "watch"},
						Resources: []string{"endpointslices"},
					},
				},
			},
		},
		{
			name: "system:controller:horizontal-pod-autoscaler",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:controller:horizontal-pod-autoscaler",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "system:controller:horizontal-pod-autoscaler",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Namespace: "kube-system",
						Name:      "horizontal-pod-autoscaler",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:controller:horizontal-pod-autoscaler",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"autoscaling"},
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{"horizontalpodautoscalers"},
					},
					{
						APIGroups: []string{"autoscaling"},
						Verbs:     []string{"update"},
						Resources: []string{"horizontalpodautoscalers/status"},
					},
					{
						APIGroups: []string{"*"},
						Verbs:     []string{"get", "update"},
						Resources: []string{"*/scale"},
					},
					// -- begin not real, just for testing
					{
						APIGroups: []string{""},
						Verbs:     []string{"get"},
						Resources: []string{"pods/*", "nodes"},
					},
					// -- end not real, just for testing
					{
						APIGroups: []string{""},
						Verbs:     []string{"list"},
						Resources: []string{"pods"},
					},
					{
						APIGroups: []string{"metrics.k8s.io"},
						Verbs:     []string{"list"},
						Resources: []string{"pods"},
					},
					{
						APIGroups: []string{"custom.metrics.k8s.io"},
						Verbs:     []string{"get", "list"},
						Resources: []string{"*"},
					},
					{
						APIGroups: []string{"external.metrics.k8s.io"},
						Verbs:     []string{"get", "list"},
						Resources: []string{"*"},
					},
					{
						APIGroups: []string{"", "events.k8s.io"},
						Verbs:     []string{"create", "patch", "update"},
						Resources: []string{"events"},
					},
				},
			},
		},
		{
			name: "crazy-policy",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "crazy-policy",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "crazy-policy",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Namespace: "default",
						Name:      "crazy-service-account",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "crazy-policy",
				},
				Rules: []rbacv1.PolicyRule{
					// list of strings for all values
					{
						APIGroups: []string{"batch", "batch.k8s.aws"},
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{"jobs", "cronjobs"},
					},
					// list of strings where one is a "*"
					// probably not valid RBAC? but we handle anyway
					{
						APIGroups: []string{"*", ""},
						Verbs:     []string{"watch", "*"},
						Resources: []string{"something"},
					},
					{
						APIGroups: []string{"*"},
						Verbs:     []string{"get", "update"},
						Resources: []string{"*/scale", "*/status"},
					},
					{
						APIGroups: []string{"", "apps"},
						Verbs:     []string{"get", "update"},
						Resources: []string{"pods/*", "nodes/status", "nodes/proxy", "deployments/*"},
					},
					{
						APIGroups: []string{""},
						Verbs:     []string{"get", "update"},
						Resources: []string{"pods/*"},
					},
					{
						APIGroups: []string{"custom.metrics.k8s.io", "external.metrics.k8s.io"},
						Verbs:     []string{"get", "list"},
						Resources: []string{"*"},
					},
					{
						APIGroups: []string{""},
						Verbs:     []string{"get"},
						Resources: []string{"pods/logs"},
					},
					{
						APIGroups:     []string{""},
						Verbs:         []string{"get"},
						Resources:     []string{"configmaps"},
						ResourceNames: []string{"aws-auth"},
					},
					{
						APIGroups:     []string{""},
						Verbs:         []string{"get"},
						Resources:     []string{"configmaps"},
						ResourceNames: []string{"kubeadm-config", "kube-proxy", "coredns"},
					},
					{
						APIGroups: []string{""},
						Verbs:     []string{"get"},
						Resources: []string{"pods", "pods/*", "node", "*"}, // nonsensical RBAC, but possible, so we handle it
					},
				},
			},
		},
		{
			name: "non-resource-url",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-resource-url-group",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "non-resource-url-group",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "Group",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "non-resource-url-actor",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-resource-url-group",
				},
				Rules: []rbacv1.PolicyRule{
					{
						NonResourceURLs: []string{"*"},
						Verbs:           []string{"get", "options"},
					},
					{
						NonResourceURLs: []string{"/healthz/*"},
						Verbs:           []string{"get"},
					},
					{
						NonResourceURLs: []string{"/readyz/*", "/version/*", "/version", "/version/"},
						Verbs:           []string{"get"},
					},
				},
			},
		},
		{
			name: "cluster-admin",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-admin",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "cluster-admin",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "Group",
						Name:     "system:masters",
						APIGroup: "rbac.authorization.k8s.io",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-admin",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"*"},
						Resources: []string{"*"},
						Verbs:     []string{"*"},
					},
					{
						NonResourceURLs: []string{"*"},
						Verbs:           []string{"*"},
					},
				},
			},
		},
		{
			name: "system:kube-controller-manager",
			crb: rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:kube-controller-manager",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "system:kube-controller-manager",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "system:kube-controller-manager",
						APIGroup: "rbac.authorization.k8s.io",
					},
				},
			},
			cr: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system:kube-controller-manager",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"*"},
						Resources: []string{"*"},
						Verbs:     []string{"list", "watch"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"servicaccount/token"},
						Verbs:     []string{"create"},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ps := ClusterRoleBindingToCedar(tc.crb, tc.cr)

			testdata := filepath.Join("testdata", tc.name+".cedar")
			if *update {
				t.Logf("updating testdata golden file %s", testdata)
				if err := os.WriteFile(testdata, ps.MarshalCedar(), 0644); err != nil {
					t.Fatalf("failed to update testdata golden file %s: %s", testdata, err)
				}
			}
			want, err := os.ReadFile(testdata)
			if err != nil {
				t.Fatalf("failed to read testdata golden file %s: %s", testdata, err)
			}

			wantPS, err := cedar.NewPolicySetFromBytes(tc.name+".cedar", want)
			if err != nil {
				t.Fatalf("failed to parse testdata golden file %s: %s", testdata, err)
			}
			if !bytes.Equal(wantPS.MarshalCedar(), ps.MarshalCedar()) {
				t.Errorf("Did not get desired polcy: %s", cmp.Diff(
					string(wantPS.MarshalCedar()),
					string(ps.MarshalCedar()),
				))
			}

		})

	}
}
