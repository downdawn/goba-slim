package app

import (
	"context"
	"fmt"

	"github.com/downdawn/goba-slim/internal/platform/health"
)

type lifecycleComponent struct {
	name   string
	start  func(context.Context) error
	stop   func(context.Context) error
	health health.Check
}

func startComponents(ctx context.Context, components []lifecycleComponent) (int, error) {
	for index, component := range components {
		if component.start == nil {
			continue
		}
		if err := component.start(ctx); err != nil {
			return index, fmt.Errorf("启动 %s: %w", component.name, err)
		}
	}
	return len(components), nil
}

func stopComponents(ctx context.Context, components []lifecycleComponent, count int) error {
	var first error
	for index := min(count, len(components)) - 1; index >= 0; index-- {
		component := components[index]
		if component.stop == nil {
			continue
		}
		if err := component.stop(ctx); err != nil && first == nil {
			first = fmt.Errorf("停止 %s: %w", component.name, err)
		}
	}
	return first
}

func componentHealthChecks(components []lifecycleComponent) map[string]health.Check {
	checks := make(map[string]health.Check, len(components))
	for _, component := range components {
		if component.health != nil {
			checks[component.name] = component.health
		}
	}
	return checks
}
