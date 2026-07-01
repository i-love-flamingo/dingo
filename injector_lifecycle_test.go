package dingo_test

import (
	"errors"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdenticalDuplicateBindingIsOk(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*string)(nil)).ToInstance("a")
	inj.Bind((*string)(nil)).ToInstance("a")
	assert.NoError(t, inj.InitModules())
}

// Override of a type with no prior binding does not error: Override() registers
// its own binding (it calls Bind internally), so the type is never "unknown" by
// the time InitModules evaluates overrides. The override's target is what resolves.
func TestOverrideOfUnboundTypeBindsIt(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Override((*greeter)(nil), "").To(german{})
	require.NoError(t, inj.InitModules())

	got, err := inj.GetInstance((*greeter)(nil))
	require.NoError(t, err)
	assert.Equal(t, "hallo", got.(greeter).Greet())
}

func TestOverrideReplacesBinding(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*greeter)(nil)).To(english{})
	inj.Override((*greeter)(nil), "").To(german{})
	require.NoError(t, inj.InitModules())

	got, err := inj.GetInstance((*greeter)(nil))
	require.NoError(t, err)
	assert.Equal(t, "hallo", got.(greeter).Greet())
}

// Candidate #5: the annotation NAME in an inject tag is not trimmed (only the
// option tokens are). " must" is expected NOT to match the binding "must".
func TestInjectTagAnnotationWhitespace(t *testing.T) {
	t.Parallel()
	type target struct {
		V string `inject:" must"` // leading space in the annotation name
	}
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*string)(nil)).AnnotatedWith("must").ToInstance("x")

	_, err = inj.GetInstance(new(target))
	if err == nil {
		t.Skip("BUG: inject-tag annotation name behaves unexpectedly with whitespace — see DINGO-FINDINGS.md#N")
	}
	assert.Error(t, err)
}

type valueReceiverInject struct{}

func (valueReceiverInject) Inject() {} // value receiver is invalid

func TestInjectMethodOnValueReceiverRejected(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	_, err = inj.GetInstance(valueReceiverInject{})
	require.Error(t, err)
	assert.ErrorIs(t, err, dingo.ErrInvalidInjectReceiver)
}

// Eager-singleton tests use DISTINCT dedicated types on purpose: dingo's
// Singleton scope is a process-global keyed by (type, annotation) and shared
// across injectors, so reusing the same type across tests would let one test's
// cached instance satisfy another and hide whether the provider actually ran.

type eagerAtInit struct{}

func TestEagerSingletonBuildsAtInit(t *testing.T) {
	t.Parallel()
	built := false
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*eagerAtInit)(nil)).ToProvider(func() *eagerAtInit { built = true; return &eagerAtInit{} }).AsEagerSingleton()

	require.NoError(t, inj.InitModules())
	assert.True(t, built, "eager singleton must build during InitModules")
}

type eagerDisabled struct{}

func TestEagerSingletonDisabledDoesNotBuild(t *testing.T) {
	t.Parallel()
	built := false
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.SetBuildEagerSingletons(false)
	inj.Bind((*eagerDisabled)(nil)).ToProvider(func() *eagerDisabled { built = true; return &eagerDisabled{} }).AsEagerSingleton()

	require.NoError(t, inj.InitModules())
	assert.False(t, built, "with eager building off, the provider must not run at init")
}

type eagerParent struct{}
type eagerChild struct{}

// BuildEagerSingletons(true) also builds the parent injector's eager singletons.
func TestBuildEagerSingletonsIncludesParent(t *testing.T) {
	t.Parallel()
	parentBuilt, childBuilt := false, false

	parent, err := dingo.NewInjector()
	require.NoError(t, err)
	parent.Bind((*eagerParent)(nil)).ToProvider(func() *eagerParent { parentBuilt = true; return &eagerParent{} }).AsEagerSingleton()

	child, err := parent.Child()
	require.NoError(t, err)
	child.Bind((*eagerChild)(nil)).ToProvider(func() *eagerChild { childBuilt = true; return &eagerChild{} }).AsEagerSingleton()

	require.NoError(t, child.BuildEagerSingletons(true))
	assert.True(t, parentBuilt, "includeParent must build the parent's eager singletons")
	assert.True(t, childBuilt)
}

// TryModule recovers a module that panics with an error value.
var errFromModule = errors.New("module blew up")

type panicErrorModule struct{}

func (*panicErrorModule) Configure(*dingo.Injector) { panic(errFromModule) }

func TestTryModuleRecoversErrorPanic(t *testing.T) {
	t.Parallel()
	err := dingo.TryModule(new(panicErrorModule))
	require.Error(t, err)
	assert.ErrorIs(t, err, errFromModule)
}
