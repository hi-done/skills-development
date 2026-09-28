package v1

import "github.com/gogf/gf/v2/frame/g"

type GetInviteCodeReq struct {
	g.Meta `path:"/user/get_invite_code" method:"get" summary:"获取邀请码"`
}

type GetInviteCodeRes struct {
	Code string `json:"code"`
}
