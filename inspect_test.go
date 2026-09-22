package dingo_test

import (
	"reflect"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	inspectedBinding struct {
		annotation string
		to         reflect.Type
		provider   *reflect.Value
		instance   *reflect.Value
		scope      dingo.Scope
	}

	inspectedMultiBinding struct {
		index       int
		annotation  string
		to          reflect.Type
		hasProvider bool
		hasInstance bool
	}

	inspectedMapBinding struct {
		key         string
		annotation  string
		to          reflect.Type
		hasProvider bool
		hasInstance bool
	}
)

// spyInspector records what the callbacks report.
//
// Assertions filter by the inspected type instead of counting raw callback
// invocations: every injector reports a dingo.Injector self-binding nobody
// bound, and a child reports it twice, because Child() binds it once through
// NewInjector and once itself.
type spyInspector struct {
	bindings      map[reflect.Type][]inspectedBinding
	multiBindings map[reflect.Type][]inspectedMultiBinding
	mapBindings   map[reflect.Type][]inspectedMapBinding
	parents       []*dingo.Injector
}

func newSpyInspector() *spyInspector {
	return &spyInspector{
		bindings:      make(map[reflect.Type][]inspectedBinding),
		multiBindings: make(map[reflect.Type][]inspectedMultiBinding),
		mapBindings:   make(map[reflect.Type][]inspectedMapBinding),
	}
}

func (s *spyInspector) inspector() dingo.Inspector {
	return dingo.Inspector{
		InspectBinding: func(of reflect.Type, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			s.bindings[of] = append(s.bindings[of], inspectedBinding{
				annotation: annotation,
				to:         to,
				provider:   provider,
				instance:   instance,
				scope:      in,
			})
		},
		InspectMultiBinding: func(of reflect.Type, index int, annotation string, to reflect.Type, provider, instance *reflect.Value, _ dingo.Scope) {
			s.multiBindings[of] = append(s.multiBindings[of], inspectedMultiBinding{
				index:       index,
				annotation:  annotation,
				to:          to,
				hasProvider: provider != nil,
				hasInstance: instance != nil,
			})
		},
		InspectMapBinding: func(of reflect.Type, key, annotation string, to reflect.Type, provider, instance *reflect.Value, _ dingo.Scope) {
			s.mapBindings[of] = append(s.mapBindings[of], inspectedMapBinding{
				key:         key,
				annotation:  annotation,
				to:          to,
				hasProvider: provider != nil,
				hasInstance: instance != nil,
			})
		},
		InspectParent: func(parent *dingo.Injector) {
			s.parents = append(s.parents, parent)
		},
	}
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}

func TestInspect_BindingCallbackReportsTargetProviderInstanceAndScope(t *testing.T) {
	t.Parallel()

	// The scope is reported as the object In received. It is not registered
	// here because nothing in this test resolves counter; which registration
	// a scope object selects is TestBindScope_FreshScopeReplacesTheCache.
	scope := dingo.NewSingletonScope()

	injector := newInjector(t)
	injector.Bind(new(greeter)).To(helloGreeter{})
	injector.Bind(new(greeter)).AnnotatedWith("loud").ToInstance(loudGreeter{})
	injector.Bind(new(counter)).In(scope)
	injector.Bind(new(string)).ToProvider(func() string { return "provided" })

	spy := newSpyInspector()
	injector.Inspect(spy.inspector())

	greeters := spy.bindings[typeOf[greeter]()]
	require.Len(t, greeters, 2)

	assert.Equal(t, "", greeters[0].annotation)
	assert.Equal(t, typeOf[helloGreeter](), greeters[0].to)
	assert.Nil(t, greeters[0].instance)
	assert.Nil(t, greeters[0].scope)

	assert.Equal(t, "loud", greeters[1].annotation)
	assert.Nil(t, greeters[1].to)
	require.NotNil(t, greeters[1].instance)
	assert.Equal(t, loudGreeter{}, greeters[1].instance.Interface())

	counters := spy.bindings[typeOf[counter]()]
	require.Len(t, counters, 1)
	assert.Same(t, scope, counters[0].scope)

	strings := spy.bindings[typeOf[string]()]
	require.Len(t, strings, 1)
	assert.NotNil(t, strings[0].provider)
}

func TestInspect_MultiBindingCallbackReportsIndexAcrossAnnotations(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.BindMulti(new(greeter)).To(helloGreeter{})
	injector.BindMulti(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})
	injector.BindMulti(new(greeter)).ToInstance(loudGreeter{})
	injector.BindMulti(new(greeter)).ToProvider(func() greeter { return loudGreeter{} })

	spy := newSpyInspector()
	injector.Inspect(spy.inspector())

	// Observed behavior, not a guarantee: the index is the position in the
	// per-type slice and is not filtered by annotation.
	assert.Equal(t, []inspectedMultiBinding{
		{index: 0, annotation: "", to: typeOf[helloGreeter]()},
		{index: 1, annotation: "loud", to: typeOf[loudGreeter]()},
		{index: 2, annotation: "", to: nil, hasInstance: true},
		{index: 3, annotation: "", to: nil, hasProvider: true},
	}, spy.multiBindings[typeOf[greeter]()])
}

func TestInspect_MapBindingCallbackFiresPerKey(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.BindMap(new(greeter), "hello").To(helloGreeter{})
	injector.BindMap(new(greeter), "loud").AnnotatedWith("loud").To(loudGreeter{})
	injector.BindMap(new(greeter), "instance").ToInstance(loudGreeter{})
	injector.BindMap(new(greeter), "provided").ToProvider(func() greeter { return loudGreeter{} })

	spy := newSpyInspector()
	injector.Inspect(spy.inspector())

	assert.ElementsMatch(t, []inspectedMapBinding{
		{key: "hello", annotation: "", to: typeOf[helloGreeter]()},
		{key: "loud", annotation: "loud", to: typeOf[loudGreeter]()},
		{key: "instance", annotation: "", to: nil, hasInstance: true},
		{key: "provided", annotation: "", to: nil, hasProvider: true},
	}, spy.mapBindings[typeOf[greeter]()])
}

func TestInspect_ParentCallbackFiresOnlyForAChild(t *testing.T) {
	t.Parallel()

	root := newInjector(t)

	rootSpy := newSpyInspector()
	root.Inspect(rootSpy.inspector())
	assert.Empty(t, rootSpy.parents)

	child := root.child(t)
	grandchild := child.child(t)

	childSpy := newSpyInspector()
	child.Inspect(childSpy.inspector())
	assert.Equal(t, []*dingo.Injector{root.Injector}, childSpy.parents)

	grandchildSpy := newSpyInspector()
	grandchild.Inspect(grandchildSpy.inspector())
	assert.Equal(t, []*dingo.Injector{child.Injector}, grandchildSpy.parents)
}

func TestInspect_NilCallbacksAreSkipped(t *testing.T) {
	t.Parallel()

	// The inspected injector owns all three kinds of binding and has a
	// parent, so every unset callback has something it would have reported.
	// Inspect never walks the parent, which is why the bindings are made on
	// the child rather than on the root.
	root := newInjector(t)
	injector := root.child(t)
	injector.Bind(new(greeter)).To(helloGreeter{})
	injector.BindMulti(new(greeter)).To(loudGreeter{})
	injector.BindMap(new(greeter), "hello").To(helloGreeter{})

	var inspected []reflect.Type

	injector.Inspect(dingo.Inspector{
		InspectBinding: func(of reflect.Type, _ string, _ reflect.Type, _, _ *reflect.Value, _ dingo.Scope) {
			inspected = append(inspected, of)
		},
	})

	assert.Contains(t, inspected, typeOf[greeter]())

	assert.NotPanics(t, func() {
		injector.Inspect(dingo.Inspector{})
	})
}

func TestInspect_ProviderAndInstanceAreNilWhenAbsent(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.Bind(new(counter))
	injector.Bind(new(greeter)).To(helloGreeter{})
	injector.Bind(new(string)).ToInstance("instance")
	injector.Bind(new(int)).ToProvider(func() int { return 1 })

	spy := newSpyInspector()
	injector.Inspect(spy.inspector())

	targetless := spy.bindings[typeOf[counter]()]
	require.Len(t, targetless, 1)
	assert.Nil(t, targetless[0].to)
	assert.Nil(t, targetless[0].provider)
	assert.Nil(t, targetless[0].instance)

	toOnly := spy.bindings[typeOf[greeter]()]
	require.Len(t, toOnly, 1)
	assert.Nil(t, toOnly[0].provider)
	assert.Nil(t, toOnly[0].instance)

	instanceOnly := spy.bindings[typeOf[string]()]
	require.Len(t, instanceOnly, 1)
	assert.Nil(t, instanceOnly[0].provider)
	assert.NotNil(t, instanceOnly[0].instance)

	providerOnly := spy.bindings[typeOf[int]()]
	require.Len(t, providerOnly, 1)
	assert.NotNil(t, providerOnly[0].provider)
	assert.Nil(t, providerOnly[0].instance)
}
