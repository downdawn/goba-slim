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
