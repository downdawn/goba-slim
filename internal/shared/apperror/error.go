// Package apperror 定义可安全映射为 HTTP 响应的应用错误。
package apperror

import "net/http"

// Error 表示具有稳定公开契约的应用错误。
type Error struct {
	Code           string
	MessageKey     string
	DefaultMessage string
	HTTPStatus     int
	Cause          error
	Details        map[string]any
	ExposeDetails  bool
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.DefaultMessage
}
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New 创建应用错误。
func New(code, messageKey, message string, status int, cause error) *Error {
	return &Error{Code: code, MessageKey: messageKey, DefaultMessage: message, HTTPStatus: status, Cause: cause}
}

// Validation 创建参数校验错误。
func Validation(code, key, message string, cause error) *Error {
	return New(code, key, message, http.StatusBadRequest, cause)
}

// NotFound 创建资源不存在错误。
func NotFound(code, key, message string, cause error) *Error {
	return New(code, key, message, http.StatusNotFound, cause)
}

// WithDetails 附加可选的安全错误详情。
func (e *Error) WithDetails(details map[string]any, expose bool) *Error {
	if e == nil {
		return nil
	}
	e.Details, e.ExposeDetails = details, expose
	return e
}
