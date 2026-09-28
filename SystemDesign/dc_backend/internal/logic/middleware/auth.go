package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"dc_backend/utility"
)

// Auth 认证中间件：校验 Access Token，过期时自动使用 Refresh Token 刷新（双 token 机制）
func (s *sMiddleware) Auth(r *ghttp.Request) {
	ctx := r.Context()

	// 校验 Access Token
	claims, err := utility.ParseAccessToken(extractAccessToken(r))
	if err == nil {
		r.SetCtxVar("userId", uint64(claims.UserID))
		r.Middleware.Next()
		return
	}

	// 非过期错误（伪造/损坏的凭证）直接拒绝
	if !errors.Is(err, utility.ErrTokenExpired) {
		r.SetError(gerror.NewCode(gcode.CodeNotAuthorized, "无效的登录凭证"))
		return
	}

	// Access Token 已过期：取 Refresh Token 尝试刷新
	refreshToken := r.Header.Get("X-Refresh-Token")
	if refreshToken == "" {
		r.SetError(gerror.NewCode(gcode.CodeNotAuthorized, "登录已过期，请重新登录"))
		return
	}

	accessToken, newRefreshToken, userId, err := refreshTokens(ctx, refreshToken)
	if err != nil {
		r.SetError(err)
		return
	}

	// 新 token 通过响应头下发给客户端
	r.Response.Header().Set("X-Access-Token", accessToken)
	r.Response.Header().Set("X-Refresh-Token", newRefreshToken)

	r.SetCtxVar("userId", userId)
	r.Middleware.Next()
}

// refreshTokens 校验 Refresh Token 并轮换签发新 token 对，返回新 token 与用户 ID
func refreshTokens(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, userId uint64, err error) {
	// 校验 Refresh Token 签名与有效期
	claims, err := utility.ParseRefreshToken(refreshToken)
	if err != nil {
		return "", "", 0, gerror.NewCode(gcode.CodeNotAuthorized, "登录已过期，请重新登录")
	}

	// 查 Redis 校验是否被吊销：不存在或与当前有效 token 不一致即视为失效
	key := fmt.Sprintf("refresh_token:%d", claims.UserID)
	v, err := g.Redis().Get(ctx, key)
	if err != nil {
		return "", "", 0, gerror.Wrap(err, "查询登录状态失败")
	}
	if v.IsNil() || v.String() != refreshToken {
		return "", "", 0, gerror.NewCode(gcode.CodeNotAuthorized, "登录状态已失效，请重新登录")
	}

	// 签发新 token 对
	accessToken, err = utility.GenerateAccessToken(claims.UserID, claims.Username)
	if err != nil {
		return "", "", 0, gerror.Wrap(err, "签发 Access Token 失败")
	}
	newRefreshToken, err = utility.GenerateRefreshToken(claims.UserID, claims.Username)
	if err != nil {
		return "", "", 0, gerror.Wrap(err, "签发 Refresh Token 失败")
	}

	// 覆盖 Redis 中的旧 Refresh Token（旧 token 立即作废），TTL 与 Refresh Token 有效期保持一致
	ttlSeconds := int64(utility.RefreshTokenExpire(ctx) / time.Second)
	if err = g.Redis().SetEX(ctx, key, newRefreshToken, ttlSeconds); err != nil {
		return "", "", 0, gerror.Wrap(err, "更新登录状态失败")
	}

	return accessToken, newRefreshToken, uint64(claims.UserID), nil
}

// extractAccessToken 从 Authorization 请求头中提取 Access Token
func extractAccessToken(r *ghttp.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
