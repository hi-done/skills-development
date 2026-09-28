// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package user

import (
	"context"

	"dc_backend/api/user/v1"
)

type IUserV1 interface {
	GetInviteCode(ctx context.Context, req *v1.GetInviteCodeReq) (res *v1.GetInviteCodeRes, err error)
}
