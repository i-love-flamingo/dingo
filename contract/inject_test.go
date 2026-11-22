package contract_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// injectMethodTarget is injected through its Inject method, which takes a
// plain parameter and a struct whose fields carry the annotations. It returns
// its receiver, the usual shape of such a method; the return value is ignored.
type injectMethodTarget struct {
	called int
	plain  greeter
	name   string
	loud   greeter
}

func (target *injectMethodTarget) Inject(plain greeter, config *struct {
	Name string  `inject:"config:shop.name"`
	Loud greeter `inject:"loud"`
},
) *injectMethodTarget {
	target.called++
	target.plain = plain
	target.name = config.Name
	target.loud = config.Loud

	return target
}

// injectingModule is a module with an Inject method that records what its
// greeter says while Configure runs.
type injectingModule struct {
	greeter         greeter
	seenInConfigure string
}

func (module *injectingModule) Inject(injected greeter) {
	module.greeter = injected
}

func (module *injectingModule) Configure(*dingo.Injector) {
	module.seenInConfigure = module.greeter.Greet()
}

// bindingModule binds a greeter.
type bindingModule struct{}

func (bindingModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(greeter)).To(helloGreeter{})
}

// Parameters of an Inject method resolve without an annotation tag, and a
// struct parameter is filled from the tags of its fields.
func TestInject_MethodParametersResolveWithoutAnnotation(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(greeter)).To(helloGreeter{})
		injector.Bind(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})
		injector.Bind(new(string)).AnnotatedWith("config:shop.name").ToInstance("basket")
	}))

	target := new(injectMethodTarget)

	require.NoError(t, injector.RequestInjection(target))

	assert.Equal(t, 1, target.called)
	assert.Equal(t, "hello", target.plain.Greet())
	assert.Equal(t, "basket", target.name)
	assert.Equal(t, "HELLO", target.loud.Greet())
}

// A module is injected before its own Configure runs, with bindings that
// earlier modules made.
func TestInject_ModuleInjectRunsBeforeConfigure(t *testing.T) {
	t.Parallel()

	module := new(injectingModule)

	newInjector(t, bindingModule{}, module)

	assert.Equal(t, "hello", module.seenInConfigure)
}
