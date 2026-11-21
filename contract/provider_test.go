package contract_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// providedService is injected with a greeter, and is the type most
	// provider tests resolve.
	providedService struct {
		Greeter greeter `inject:""`
	}

	providedServiceProvider func() *providedService

	greeterSliceProvider func() []greeter

	greeterMapProvider func() map[string]greeter

	// providerFields asks for one provider of each kind.
	providerFields struct {
		Service providedServiceProvider `inject:""`
		Slice   greeterSliceProvider    `inject:""`
		Map     greeterMapProvider      `inject:""`
	}

	// providerResult is built by a provider function and injected afterwards.
	providerResult struct {
		FromArgument greeter
		Field        greeter `inject:"loud"`
	}
)

// A provider field of a type whose name ends in Provider calls the resolution
// on every call. Without a scope, each call gives a new, injected instance,
// for a struct as well as for a multibinding and a map binding.
func TestProvider_UnscopedBindingGivesAFreshInjectedInstancePerCall(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(greeter)).To(helloGreeter{})
		injector.BindMulti(new(greeter)).To(helloGreeter{})
		injector.BindMap(new(greeter), "shop").To(helloGreeter{})
	}))

	var fields providerFields

	require.NoError(t, injector.RequestInjection(&fields))

	firstService, secondService := fields.Service(), fields.Service()
	assert.NotSame(t, firstService, secondService)
	assert.Equal(t, "hello", firstService.Greeter.Greet())

	firstSlice, secondSlice := fields.Slice(), fields.Slice()
	require.Len(t, firstSlice, 1)
	require.Len(t, secondSlice, 1)
	assert.NotSame(t, firstSlice[0], secondSlice[0])

	firstMap, secondMap := fields.Map(), fields.Map()
	require.Contains(t, firstMap, "shop")
	require.Contains(t, secondMap, "shop")
	assert.NotSame(t, firstMap["shop"], secondMap["shop"])
}

// A provider of a scoped binding gives the scoped instance.
func TestProvider_ScopedBindingGivesTheScopedInstance(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(greeter)).To(helloGreeter{})
		injector.Bind(new(providedService)).In(dingo.Singleton)
	}))

	var fields providerFields

	require.NoError(t, injector.RequestInjection(&fields))

	resolved, err := injector.GetInstance(new(providedService))
	require.NoError(t, err)

	service := fields.Service()

	assert.Same(t, resolved, service)
	assert.Same(t, service, fields.Service())
}

// The provider field is filled when it is requested, and an error in its
// resolution can only be reported on the call, where it is a panic.
func TestLatent_ProviderPanicsWhenResolutionFails(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	var target greeterProviderField

	require.NoError(t, injector.RequestInjection(&target))

	assert.Panics(t, func() {
		target.Provide()
	})
}

// The arguments of a provider function are resolved by their type, and what
// the function returns is injected before the caller gets it.
func TestProvider_ToProviderArgumentsAreResolvedAndTheResultIsInjected(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(greeter)).To(helloGreeter{})
		injector.Bind(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})
		injector.Bind(new(providerResult)).ToProvider(func(argument greeter) *providerResult {
			return &providerResult{FromArgument: argument}
		})
	}))

	resolved, err := injector.GetInstance(new(providerResult))
	require.NoError(t, err)

	result := resolved.(*providerResult)

	assert.Equal(t, "hello", result.FromArgument.Greet())
	assert.Equal(t, "HELLO", result.Field.Greet())
}
