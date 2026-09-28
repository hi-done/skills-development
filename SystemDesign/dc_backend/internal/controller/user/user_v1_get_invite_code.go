package user

import (
	"context"

	v1 "dc_backend/api/user/v1"
	"dc_backend/internal/service"
)

func (c *ControllerV1) GetInviteCode(ctx context.Context, req *v1.GetInviteCodeReq) (res *v1.GetInviteCodeRes, err error) {
	code, err := service.User().GetInviteCode(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.GetInviteCodeRes{Code: code}, nil
}
