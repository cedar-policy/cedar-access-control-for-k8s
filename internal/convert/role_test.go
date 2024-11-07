package convert

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRoleBindingToCedar(t *testing.T) {
	// TODO: test case cleanup
	// * read testcases from input files containing CRB and CR objects
	// * add all of Kind's in-cluster roles/roleBindings
	// * migrate custom policies defined below to input files

	testCases := []struct {
		name        string
		roleBinding rbacv1.RoleBinding
		role        rbacv1.Role
	}{
		{
			name: "system:controller:token-cleaner",
			roleBinding: rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "system:controller:token-cleaner",
					Namespace: "kube-system",
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "Role",
					Name: "system:controller:token-cleaner",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "token-cleaner",
						Namespace: "kube-system",
					},
				},
			},
			role: rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "system:controller:token-cleaner",
					Namespace: "kube-system",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"secrets"},
						Verbs:     []string{"delete", "get", "list", "watch"},
					},
					{
						APIGroups: []string{"", "events.k8s.io"},
						Verbs:     []string{"create", "patch", "update"},
						Resources: []string{"events"},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ps := RoleBindingToCedar(tc.roleBinding, tc.role)

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
