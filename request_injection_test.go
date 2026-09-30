package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type greetingField struct {
	Greeting string `inject:"greeting"`
}

// requestingModule asks for injection while it is being configured and records
// what the object held at that moment.
type requestingModule struct {
	target          *greetingField
	seenInConfigure string
}

func (m *requestingModule) Configure(injector *dingo.Injector) {
	m.target = new(greetingField)

	if err := injector.RequestInjection(m.target); err != nil {
		panic(err)
	}

	m.seenInConfigure = m.target.Greeting
}

// greetingModule binds the annotation requestingModule's target needs, and is
// registered after it.
type greetingModule struct{}

func (greetingModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("hello")
}

// unresolvableField cannot be injected: nothing binds greeter.
type unresolvableField struct {
	Greeter greeter `inject:""`
}

type queuingFailureModule struct{}

func (queuingFailureModule) Configure(injector *dingo.Injector) {
	if err := injector.RequestInjection(new(unresolvableField)); err != nil {
		panic(err)
	}
}

func TestRequestInjection_DelayedDuringConfigureRunsAfterAllModules(t *testing.T) {
	t.Parallel()

	module := &requestingModule{}

	newInjector(t, module, greetingModule{})

	assert.Empty(t, module.seenInConfigure)
	assert.Equal(t, "hello", module.target.Greeting)
}

func TestRequestInjection_AfterInitModulesInjectsImmediately(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, greetingModule{})

	target := new(greetingField)
	require.NoError(t, injector.RequestInjection(target))

	assert.Equal(t, "hello", target.Greeting)
}

func TestRequestInjection_QueuedFailureReturnsFromInitModules(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: the queued injection error is
	// returned as it is, so a caller matching ErrInitModules does not see it.
	// The absence of that wrapping is not asserted, because wrapping it would
	// be an improvement.
	injector := newInjector(t)

	err := injector.InitModules(queuingFailureModule{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "can not instantiate interface")
}
