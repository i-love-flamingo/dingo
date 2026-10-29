package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingEagerModule binds an eager singleton whose provider appends to a
// shared log, so that the build can be observed without resolving anything.
type recordingEagerModule struct {
	name string
	log  *[]string
}

func (m *recordingEagerModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(counter)).ToProvider(func() *counter {
		*m.log = append(*m.log, m.name)

		return &counter{N: len(*m.log)}
	}).AsEagerSingleton()
}

// recordingChildEagerModule binds a different type than recordingEagerModule,
// so that a child and its parent can each have an eager singleton of their own.
type recordingChildEagerModule struct {
	log *[]string
}

func (m *recordingChildEagerModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(helloGreeter)).ToProvider(func() *helloGreeter {
		*m.log = append(*m.log, "child")

		return &helloGreeter{Name: "child"}
	}).AsEagerSingleton()
}

func TestSetBuildEagerSingletons_DisablesTheAutomaticBuild(t *testing.T) {
	t.Parallel()

	var log []string

	injector := newInjector(t)
	injector.SetBuildEagerSingletons(false)
	require.NoError(t, injector.InitModules(&recordingEagerModule{name: "eager", log: &log}))

	assert.Empty(t, log)

	require.NoError(t, injector.BuildEagerSingletons(false))

	assert.Equal(t, []string{"eager"}, log)
}

func TestEagerSingleton_IsBuiltByInitModules(t *testing.T) {
	t.Parallel()

	var log []string

	newInjector(t, &recordingEagerModule{name: "eager", log: &log})

	assert.Equal(t, []string{"eager"}, log)
}

func TestEagerSingleton_BuildsWithoutTouchingParent(t *testing.T) {
	t.Parallel()

	var log []string

	parent := newInjector(t)
	parent.SetBuildEagerSingletons(false)
	require.NoError(t, parent.InitModules(&recordingEagerModule{name: "parent", log: &log}))

	child := parent.child(t)
	child.SetBuildEagerSingletons(false)
	require.NoError(t, child.InitModules(&recordingChildEagerModule{log: &log}))

	require.NoError(t, child.BuildEagerSingletons(false))

	assert.Equal(t, []string{"child"}, log)
}

func TestEagerSingleton_WithParentBuildsChildBeforeParent(t *testing.T) {
	t.Parallel()

	var log []string

	parent := newInjector(t)
	parent.SetBuildEagerSingletons(false)
	require.NoError(t, parent.InitModules(&recordingEagerModule{name: "parent", log: &log}))

	child := parent.child(t)
	child.SetBuildEagerSingletons(false)
	require.NoError(t, child.InitModules(&recordingChildEagerModule{log: &log}))

	require.NoError(t, child.BuildEagerSingletons(true))

	assert.Equal(t, []string{"child", "parent"}, log)
}

func TestEagerSingleton_ConstructionFailureIsReportedByInitModules(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	err := injector.InitModules(failingEagerModule{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "loading eager singletons")
	assert.ErrorContains(t, err, "can not instantiate interface")

	// Observed behavior, not a guarantee: TryModule accepts the same module,
	// because it proves bind-time acceptance and never builds eager
	// singletons.
	assert.NoError(t, dingo.TryModule(failingEagerModule{}))
}
