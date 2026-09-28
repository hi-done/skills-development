package user

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"dc_backend/utility"
)

func (s *sUser) GetInviteCode(ctx context.Context) (string, error) {
	userId, ok := ctx.Value("userId").(uint64)
	if !ok {
		return "", gerror.NewCode(gcode.CodeNotAuthorized, "用户未登录")
	}
	// 生成随机六位邀请码
	code := utility.GenerateRandomCode(6)
	// 写入redis, key: invite_code:userId, value: code
	key := fmt.Sprintf("invite_code:%s", code)
	value := fmt.Sprintf("%d", userId)
	// 有效时间 1 天
	ttlSeconds := int64(24 * time.Hour / time.Second)
	// 创建失败返回错误
	if err := g.Redis().SetEX(ctx, key, value, ttlSeconds); err != nil {
		return "", gerror.Wrap(err, "写入邀请码失败")
	}
	return code, nil
}
