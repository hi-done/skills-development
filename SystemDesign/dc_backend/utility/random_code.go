package utility

import "github.com/gogf/gf/v2/util/grand"

// GenerateRandomCode 生成随机码，长度为 size
func GenerateRandomCode(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return grand.Str(charset, size)
}
