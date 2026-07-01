package dingo_test

import (
	"reflect"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectVisitsBindings(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*greeter)(nil)).To(english{})
	inj.BindMulti((*greeter)(nil)).ToInstance(german{})
	inj.BindMap((*greeter)(nil), "k").ToInstance(english{})

	greeterType := reflect.TypeOf((*greeter)(nil)).Elem()
	var sawBinding, sawMulti, sawMap bool

	inj.Inspect(dingo.Inspector{
		InspectBinding: func(of reflect.Type, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			if of == greeterType {
				sawBinding = true
			}
		},
		InspectMultiBinding: func(of reflect.Type, index int, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			if of == greeterType {
				sawMulti = true
			}
		},
		InspectMapBinding: func(of reflect.Type, key, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			if of == greeterType && key == "k" {
				sawMap = true
			}
		},
	})

	assert.True(t, sawBinding, "InspectBinding must see the greeter binding")
	assert.True(t, sawMulti, "InspectMultiBinding must see the greeter multibinding")
	assert.True(t, sawMap, "InspectMapBinding must see the greeter map binding")
}

func TestInspectVisitsParent(t *testing.T) {
	t.Parallel()
	parent, err := dingo.NewInjector()
	require.NoError(t, err)
	child, err := parent.Child()
	require.NoError(t, err)

	var seen *dingo.Injector
	child.Inspect(dingo.Inspector{
		InspectParent: func(p *dingo.Injector) { seen = p },
	})
	assert.Same(t, parent, seen, "InspectParent must receive the parent injector")
}
