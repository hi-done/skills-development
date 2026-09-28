package user

import "dc_backend/internal/service"

type sUser struct{}

func init() {
	service.RegisterUser(New())
}
func New() service.IUser {
	return &sUser{}
}
