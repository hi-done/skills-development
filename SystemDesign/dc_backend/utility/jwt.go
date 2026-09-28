package utility

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// 默认有效期，可通过配置文件 jwt.access_expire / jwt.refresh_expire 调整
const (
	defaultAccessTokenExpire  = 3 * time.Minute
	defaultRefreshTokenExpire = 7 * 24 * time.Hour
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成 Access Token
func GenerateAccessToken(userID int64, username string) (string, error) {
	ctx := gctx.New()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpire(ctx, "jwt.access_expire", defaultAccessTokenExpire))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	as, _ := g.Cfg().Get(ctx, "jwt.access_secret")
	return token.SignedString([]byte(as.String()))
}

// ParseAccessToken 解析 Access Token
func ParseAccessToken(tokenStr string) (*Claims, error) {
	ctx := gctx.New()
	as, _ := g.Cfg().Get(ctx, "jwt.access_secret")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(as.String()), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// GenerateRefreshToken 生成 Refresh Token
func GenerateRefreshToken(userID int64, username string) (string, error) {
	ctx := gctx.New()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpire(ctx, "jwt.refresh_expire", defaultRefreshTokenExpire))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	rs, _ := g.Cfg().Get(ctx, "jwt.refresh_secret")
	return token.SignedString([]byte(rs.String()))
}

// ParseRefreshToken 解析 Refresh Token
func ParseRefreshToken(tokenStr string) (*Claims, error) {
	ctx := gctx.New()
	rs, _ := g.Cfg().Get(ctx, "jwt.refresh_secret")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(rs.String()), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// RefreshTokenExpire 返回 Refresh Token 有效期，供刷新时同步 Redis TTL
func RefreshTokenExpire(ctx context.Context) time.Duration {
	return tokenExpire(ctx, "jwt.refresh_expire", defaultRefreshTokenExpire)
}

// tokenExpire 读取指定配置项作为有效期，配置缺失或无效时回退默认值
func tokenExpire(ctx context.Context, configKey string, def time.Duration) time.Duration {
	expire := g.Cfg().MustGet(ctx, configKey, def).Duration()
	if expire <= 0 {
		return def
	}
	return expire
}
