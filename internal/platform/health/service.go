// Package health 提供进程存活与依赖就绪检查。
package health

import "context"

// Status 表示健康检查状态。
type Status string

const (
	StatusOK   Status = "ok"
	StatusDown Status = "down"
)

// Check 是一个可取消的就绪检查。
type Check func(context.Context) error
type CheckResult struct{ Status Status }
type Liveness struct{ Status Status }
type Readiness struct {
	Ready  bool
	Checks map[string]CheckResult
}

// Service 组合所有已启用依赖检查。
type Service struct{ checks map[string]Check }

func NewService(checks map[string]Check) *Service {
	copied := make(map[string]Check, len(checks))
	for name, check := range checks {
		copied[name] = check
	}
	return &Service{checks: copied}
}
func (s *Service) Live() Liveness { return Liveness{Status: StatusOK} }
func (s *Service) Ready(ctx context.Context) Readiness {
	result := Readiness{Ready: true, Checks: make(map[string]CheckResult, len(s.checks))}
	for name, check := range s.checks {
		if check == nil || check(ctx) != nil {
			result.Ready = false
			result.Checks[name] = CheckResult{Status: StatusDown}
			continue
		}
		result.Checks[name] = CheckResult{Status: StatusOK}
	}
	return result
}
