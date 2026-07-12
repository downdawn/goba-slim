package user

import "errors"

var (
	ErrNotFound         = errors.New("用户不存在")
	ErrUsernameConflict = errors.New("用户名已存在")
	ErrEmailConflict    = errors.New("邮箱已存在")
	ErrLastSuperuser    = errors.New("必须保留至少一个可用超级管理员")
	ErrInvalidInput     = errors.New("用户输入无效")
	ErrInvalidPassword  = errors.New("密码不符合安全策略")
	ErrPasswordMismatch = errors.New("原密码错误")
)
