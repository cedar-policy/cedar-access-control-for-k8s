package entities

import (
	"strings"

	"github.com/awslabs/cedar-access-control-for-k8s/internal/schema"
	cedartypes "github.com/cedar-policy/cedar-go/types"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

type UserInfoWrapper struct {
	authenticationv1.UserInfo
}

var _ user.Info = &UserInfoWrapper{}

func (u *UserInfoWrapper) GetName() string { return u.UserInfo.Username }
func (u *UserInfoWrapper) GetUID() string {
	// set a userID if not present so we can identifiy the user entity
	if u.UserInfo.UID == "" {
		return u.UserInfo.Username
	}
	return u.UserInfo.UID
}
func (u *UserInfoWrapper) GetGroups() []string { return u.UserInfo.Groups }
func (u *UserInfoWrapper) GetExtra() map[string][]string {
	resp := map[string][]string{}
	for k, v := range u.UserInfo.Extra {
		resp[k] = []string(v)
	}
	return resp
}

func UserToCedarEntity(user user.Info) (cedartypes.EntityUID, cedartypes.Entities) {
	resp := cedartypes.Entities{}

	groupEntityUids := []cedartypes.EntityUID{}

	for _, group := range user.GetGroups() {
		groupEntityUid := cedartypes.EntityUID{
			Type: schema.GroupEntityType,
			ID:   cedartypes.String(group),
		}
		groupEntity := cedartypes.Entity{
			UID: groupEntityUid,
			Attributes: cedartypes.NewRecord(cedartypes.RecordMap(map[cedartypes.String]cedartypes.Value{
				cedartypes.String("name"): cedartypes.String(group),
			})),
		}

		resp[groupEntityUid] = &groupEntity
		groupEntityUids = append(groupEntityUids, groupEntityUid)
	}

	attributes := cedartypes.RecordMap(map[cedartypes.String]cedartypes.Value{
		cedartypes.String("name"): cedartypes.String(user.GetName()),
		// Groups field is omitted since we represent groups as entities
	})

	principalEntityType := schema.UserEntityType
	if strings.HasPrefix(user.GetName(), "system:node:") && strings.Count(user.GetName(), ":") == 2 {
		principalEntityType = schema.NodeEntityType
		attributes[cedartypes.String("name")] = cedartypes.String(strings.Split(user.GetName(), ":")[2])
	}

	if strings.HasPrefix(user.GetName(), "system:serviceaccount:") && strings.Count(user.GetName(), ":") == 3 {
		principalEntityType = schema.ServiceAccountEntityType
		parts := strings.Split(user.GetName(), ":")
		attributes[cedartypes.String("namespace")] = cedartypes.String(parts[2])
		attributes[cedartypes.String("name")] = cedartypes.String(parts[3])
	}

	// TODO: ENTITY TAGS: use entity tags once supported
	extraValues := []cedartypes.Value{}
	for k, v := range user.GetExtra() {
		extraVV := []cedartypes.Value{}
		for _, vv := range v {
			extraVV = append(extraVV, cedartypes.String(vv))
		}
		extraValues = append(extraValues, cedartypes.NewRecord(cedartypes.RecordMap{
			"key":    cedartypes.String(k),
			"values": cedartypes.NewSet(extraVV),
		}))
	}
	if len(extraValues) > 0 {
		attributes["extra"] = cedartypes.NewSet(extraValues)
	}

	principalUID := cedartypes.EntityUID{
		Type: principalEntityType,
		ID:   cedartypes.String(user.GetUID()),
	}

	resp[principalUID] = &cedartypes.Entity{
		UID:        principalUID,
		Attributes: cedartypes.NewRecord(attributes),
		Parents:    groupEntityUids,
	}
	return principalUID, resp
}
