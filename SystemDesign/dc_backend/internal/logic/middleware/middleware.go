package middleware

import "dc_backend/internal/service"

type sMiddleware struct{}

func init() {
	service.RegisterMiddleware(New())
}
func New() service.IMiddleware {
	return &sMiddleware{}
}
