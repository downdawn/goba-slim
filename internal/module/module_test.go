package module

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type testModule struct{ manifest Manifest }

func (m testModule) Manifest() Manifest     { return m.manifest }
func (testModule) Register(*Registry) error { return nil }

func TestRegistryResolvesDependenciesBeforeDependents(t *testing.T) {
	registry := NewRegistry()
	require.NoError(t, registry.Add(testModule{manifest: Manifest{Name: "api", Requires: []string{"database"}}}))
	require.NoError(t, registry.Add(testModule{manifest: Manifest{Name: "database"}}))

	ordered, err := registry.Resolve(nil)

	require.NoError(t, err)
	require.Equal(t, []string{"database", "api"}, []string{ordered[0].Manifest().Name, ordered[1].Manifest().Name})
}

func TestRegistryRejectsInvalidModuleGraphs(t *testing.T) {
	tests := []struct {
		name string
		add  func(*Registry) error
		want string
	}{
		{
			name: "duplicate module",
			add: func(registry *Registry) error {
				require.NoError(t, registry.Add(testModule{manifest: Manifest{Name: "api"}}))
				return registry.Add(testModule{manifest: Manifest{Name: "api"}})
			},
			want: "已注册",
		},
		{
			name: "missing dependency",
			add: func(registry *Registry) error {
				require.NoError(t, registry.Add(testModule{manifest: Manifest{Name: "api", Requires: []string{"database"}}}))
				_, err := registry.Resolve(nil)
				return err
			},
			want: "未注册依赖模块",
		},
		{
			name: "cyclic dependency",
			add: func(registry *Registry) error {
				require.NoError(t, registry.Add(testModule{manifest: Manifest{Name: "api", Requires: []string{"database"}}}))
				require.NoError(t, registry.Add(testModule{manifest: Manifest{Name: "database", Requires: []string{"api"}}}))
				_, err := registry.Resolve(nil)
				return err
			},
			want: "模块依赖存在循环",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorContains(t, test.add(NewRegistry()), test.want)
		})
	}
}

func TestRuntimeStopsStartedModulesInReverseOrder(t *testing.T) {
	events := []string{}
	module := lifecycleTestModule{manifest: Manifest{Name: "worker"}, events: &events}
	runtime := NewRuntime([]Module{module})

	require.NoError(t, runtime.Start(context.Background()))
	require.NoError(t, runtime.Stop(context.Background()))
	require.Equal(t, []string{"start:worker", "stop:worker"}, events)
}

type lifecycleTestModule struct {
	manifest Manifest
	events   *[]string
}

func (m lifecycleTestModule) Manifest() Manifest     { return m.manifest }
func (lifecycleTestModule) Register(*Registry) error { return nil }
func (m lifecycleTestModule) Start(context.Context) error {
	*m.events = append(*m.events, "start:"+m.manifest.Name)
	return nil
}
func (m lifecycleTestModule) Stop(context.Context) error {
	*m.events = append(*m.events, "stop:"+m.manifest.Name)
	return nil
}
