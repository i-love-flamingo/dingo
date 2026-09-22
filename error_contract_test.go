package dingo_test

import (
	"errors"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errConfigure is what panicModule panics with, to show TryModule returns the
// very same error.
var errConfigure = errors.New("configure failed")

type panicModule struct{}

func (panicModule) Configure(*dingo.Injector) {
	panic(errConfigure)
}

type panicWithNonErrorModule struct{}

func (panicWithNonErrorModule) Configure(*dingo.Injector) {
	panic(42)
}

type (
	// cyclingModule and cycledModule depend on each other.
	cyclingModule struct{}
	cycledModule  struct{}
)

func (cyclingModule) Configure(*dingo.Injector) {}

func (cyclingModule) Depends() []dingo.Module {
	return []dingo.Module{cycledModule{}}
}

func (cycledModule) Configure(*dingo.Injector) {}

func (cycledModule) Depends() []dingo.Module {
	return []dingo.Module{cyclingModule{}}
}

// valueReceiverInjectGreeter has an Inject method on a value receiver, which
// the engine rejects.
type valueReceiverInjectGreeter struct{}

func (valueReceiverInjectGreeter) Greet() string {
	return "hello"
}

func (valueReceiverInjectGreeter) Inject() {}

// structKindField has an inject-tagged field of struct kind, which the engine
// rejects whether or not the type is bound.
type structKindField struct {
	Counter counter `inject:""`
}

func TestGetInstance_UnboundInterfaceErrors(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	_, err := injector.GetInstance(new(greeter))

	require.Error(t, err)
	assert.ErrorContains(t, err, "can not instantiate interface")
	assert.ErrorContains(t, err, "greeter")
}

func TestInjection_ValueReceiverInjectMethodReturnsTheSentinel(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.Bind(new(greeter)).To(valueReceiverInjectGreeter{})

	_, err := injector.GetInstance(new(greeter))

	require.Error(t, err)
	assert.ErrorIs(t, err, dingo.ErrInvalidInjectReceiver)
}

func TestInjection_StructKindFieldIsAnError(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: an inject-tagged field of struct
	// kind is rejected whether or not that type is bound.

	t.Run("unbound", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)

		_, err := injector.GetInstance(new(structKindField))

		require.Error(t, err)
		assert.ErrorContains(t, err, "can not inject into struct")
	})

	t.Run("bound", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)
		injector.Bind(new(counter)).ToInstance(&counter{N: 1})

		_, err := injector.GetInstance(new(structKindField))

		require.Error(t, err)
		assert.ErrorContains(t, err, "can not inject into struct")
	})
}

func TestTryModule_PassesThroughErrorPanicsUnchanged(t *testing.T) {
	t.Parallel()

	err := dingo.TryModule(panicModule{})

	require.Error(t, err)
	assert.ErrorIs(t, err, errConfigure)
}

func TestTryModule_WrapsNonErrorPanicsWithQFormatting(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: the message is %q of the panic
	// value, so an int renders as a character literal. Asserted verbatim
	// because the surprise is the point, not because the format is fixed.
	err := dingo.TryModule(panicWithNonErrorModule{})

	require.Error(t, err)
	assert.EqualError(t, err, "dingo.TryModule panic: '*'")
}

func TestInitModules_CycleWrapsBothSentinels(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	err := injector.InitModules(cyclingModule{})

	require.Error(t, err)
	assert.ErrorIs(t, err, dingo.ErrInitModules)
	assert.ErrorIs(t, err, dingo.ErrModuleCycle)
	assert.ErrorContains(t, err, "cyclingModule → ")
}
