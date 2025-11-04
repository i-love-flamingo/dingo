package dingo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test types for binding_v2 functions
type (
	// Base interfaces
	v2TestInterface interface {
		Method() string
	}

	v2TestSubInterface interface {
		v2TestInterface
		SubMethod() int
	}

	// Implementations
	v2TestImpl1 struct {
		value string
	}

	v2TestImpl2 struct {
		number int
	}

	v2TestSubImpl struct {
		str string
		num int
	}

	// Pointer-based types
	v2TestPtrImpl struct {
		data string
	}

	// Generic provider types
	v2TestProvider          func() v2TestInterface
	v2TestProviderWithError func() (v2TestInterface, error)

	// Dependency injection test structs
	v2InjectionTest struct {
		Basic     v2TestInterface `inject:""`
		Annotated v2TestInterface `inject:"annotated"`
		Provider  v2TestProvider  `inject:""`
	}

	// Multi-binding test types
	v2MultiBindInterface interface {
		GetValue() string
	}

	v2MultiImpl1 struct{}
	v2MultiImpl2 struct{}
	v2MultiImpl3 struct{}

	v2MultiInjectionTest struct {
		Multi []v2MultiBindInterface `inject:""`
	}
)

func (i *v2TestImpl1) Method() string {
	return i.value
}

func (i *v2TestImpl2) Method() string {
	return "impl2"
}

func (i *v2TestSubImpl) Method() string {
	return i.str
}

func (i *v2TestSubImpl) SubMethod() int {
	return i.num
}

func (i *v2TestPtrImpl) Method() string {
	return i.data
}

func (i *v2MultiImpl1) GetValue() string {
	return "multi1"
}

func (i *v2MultiImpl2) GetValue() string {
	return "multi2"
}

func (i *v2MultiImpl3) GetValue() string {
	return "multi3"
}

// TestBind_BasicBinding tests the basic functionality of Bind[T, U]
func TestBind_BasicBinding(t *testing.T) {
	t.Run("should bind interface to implementation", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, v2TestImpl1](injector)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf((*v2TestInterface)(nil)).Elem(), binding.typeof)
		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.to)
	})

	t.Run("should bind struct to struct", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestImpl1, v2TestImpl1](injector)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.typeof)
		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.to)
	})

	t.Run("should add binding to injector bindings map", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector)

		typ := reflect.TypeOf((*v2TestInterface)(nil)).Elem()
		assert.Contains(t, injector.bindings, typ)
		assert.Len(t, injector.bindings[typ], 1)
	})

	t.Run("should allow multiple bindings of same type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector)
		Bind[v2TestInterface, v2TestImpl2](injector)

		typ := reflect.TypeOf((*v2TestInterface)(nil)).Elem()
		assert.Len(t, injector.bindings[typ], 2)
	})
}

// TestBind_PointerHandling tests pointer handling in Bind[T, U]
func TestBind_PointerHandling(t *testing.T) {
	t.Run("should strip pointer from T type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[*v2TestInterface, v2TestImpl1](injector)

		assert.Equal(t, reflect.TypeOf((*v2TestInterface)(nil)).Elem(), binding.typeof)
	})

	t.Run("should strip pointer from U type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, *v2TestImpl1](injector)

		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.to)
	})

	t.Run("should strip multiple levels of pointers from U", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		type PtrPtr **v2TestImpl1
		binding := Bind[v2TestInterface, PtrPtr](injector)

		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.to)
	})

	t.Run("should bind interface to pointer implementation", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, *v2TestPtrImpl](injector)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf(v2TestPtrImpl{}), binding.to)
	})
}

// TestBind_AssignabilityValidation tests type assignability validation
func TestBind_AssignabilityValidation(t *testing.T) {
	t.Run("should panic when U is not assignable to T", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.Panics(t, func() {
			Bind[v2TestInterface, int](injector)
		})
	})

	t.Run("should panic with correct error message", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, r, "not assignable to")
			}
		}()

		Bind[v2TestInterface, string](injector)
	})

	t.Run("should allow pointer to struct assignable to interface", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			Bind[v2TestInterface, *v2TestPtrImpl](injector)
		})
	})

	t.Run("should allow sub-interface assignable to parent interface", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			Bind[v2TestInterface, v2TestSubInterface](injector)
		})
	})
}

// TestBind_ChainableMethods tests that binding methods can be chained
func TestBind_ChainableMethods(t *testing.T) {
	t.Run("should support method chaining with AnnotatedWith", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, v2TestImpl1](injector).AnnotatedWith("test")

		assert.Equal(t, "test", binding.annotatedWith)
	})

	t.Run("should support method chaining with In", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, v2TestImpl1](injector).In(Singleton)

		assert.Equal(t, Singleton, binding.scope)
	})

	t.Run("should support method chaining with AsEagerSingleton", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, v2TestImpl1](injector).AsEagerSingleton()

		assert.True(t, binding.eager)
		assert.Equal(t, Singleton, binding.scope)
	})

	t.Run("should support complex method chaining", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := Bind[v2TestInterface, v2TestImpl1](injector).
			AnnotatedWith("complex").
			In(Singleton)

		assert.Equal(t, "complex", binding.annotatedWith)
		assert.Equal(t, Singleton, binding.scope)
	})
}

// TestBindFor_BasicBinding tests the basic functionality of BindFor[T]
func TestBindFor_BasicBinding(t *testing.T) {
	t.Run("should bind interface using runtime value", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := &v2TestImpl1{value: "test"}
		binding := BindFor[v2TestInterface](injector, impl)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf((*v2TestInterface)(nil)).Elem(), binding.typeof)
		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.to)
	})

	t.Run("should bind using concrete struct value", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := v2TestImpl2{number: 123}
		binding := BindFor[v2TestInterface](injector, &impl)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf(v2TestImpl2{}), binding.to)
	})

	t.Run("should add binding to injector", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := &v2TestImpl1{value: "test"}
		BindFor[v2TestInterface](injector, impl)

		typ := reflect.TypeOf((*v2TestInterface)(nil)).Elem()
		assert.Contains(t, injector.bindings, typ)
		assert.Len(t, injector.bindings[typ], 1)
	})
}

// TestBindFor_PointerHandling tests pointer handling in BindFor[T]
func TestBindFor_PointerHandling(t *testing.T) {
	t.Run("should strip pointer from T type parameter", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		// Use a pointer type
		str := "test"
		ptr := &str
		binding := BindFor[*string](injector, ptr)

		assert.Equal(t, reflect.TypeOf(""), binding.typeof)
	})

	t.Run("should strip pointer from value type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := &v2TestImpl1{value: "test"}
		binding := BindFor[v2TestInterface](injector, impl)

		assert.Equal(t, reflect.TypeOf(v2TestImpl1{}), binding.to)
	})

	t.Run("should handle value bound to non-pointer type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		str := "test"
		binding := BindFor[string](injector, str)

		assert.Equal(t, reflect.TypeOf(""), binding.to)
	})
}

// TestBindFor_AssignabilityValidation tests type assignability validation for BindFor
func TestBindFor_AssignabilityValidation(t *testing.T) {
	t.Run("should allow compatible value types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			BindFor[v2TestInterface](injector, &v2TestImpl1{value: "ok"})
		})
	})

	t.Run("should allow exact type match", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			BindFor[string](injector, "test string")
		})
	})

	t.Run("should allow pointer types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			str := "test"
			BindFor[*string](injector, &str)
		})
	})
}

// TestBindInstance_BasicBinding tests the basic functionality of BindInstance[T]
func TestBindInstance_BasicBinding(t *testing.T) {
	t.Run("should bind instance directly", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := &v2TestImpl1{value: "singleton"}
		binding := BindInstance[v2TestInterface](injector, impl)

		assert.NotNil(t, binding)
		assert.NotNil(t, binding.instance)
		assert.Equal(t, reflect.TypeOf(impl), binding.instance.itype)
		assert.Equal(t, reflect.ValueOf(impl), binding.instance.ivalue)
	})

	t.Run("should bind concrete struct instance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := v2TestImpl1{value: "test"}
		binding := BindInstance[v2TestImpl1](injector, impl)

		assert.NotNil(t, binding)
		assert.NotNil(t, binding.instance)
		assert.Equal(t, reflect.TypeOf(impl), binding.instance.itype)
	})

	t.Run("should bind string instance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		str := "test string"
		binding := BindInstance[string](injector, str)

		assert.NotNil(t, binding)
		assert.Equal(t, str, binding.instance.ivalue.Interface())
	})

	t.Run("should add binding to injector", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := &v2TestImpl1{value: "test"}
		BindInstance[v2TestInterface](injector, impl)

		typ := reflect.TypeOf((*v2TestInterface)(nil)).Elem()
		assert.Contains(t, injector.bindings, typ)
		assert.Len(t, injector.bindings[typ], 1)
	})
}

// TestBindInstance_PointerHandling tests pointer handling in BindInstance[T]
func TestBindInstance_PointerHandling(t *testing.T) {
	t.Run("should strip pointer from T type parameter", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		str := "test"
		ptr := &str
		binding := BindInstance[*string](injector, ptr)

		assert.Equal(t, reflect.TypeOf(""), binding.typeof)
	})

	t.Run("should preserve instance type with pointer", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		impl := &v2TestImpl1{value: "test"}
		binding := BindInstance[v2TestInterface](injector, impl)

		assert.Equal(t, reflect.TypeOf(impl), binding.instance.itype)
	})

	t.Run("should handle value instance for primitive types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		num := 42
		binding := BindInstance[int](injector, num)

		assert.Equal(t, reflect.TypeOf(num), binding.instance.itype)
	})
}

// TestBindInstance_AssignabilityValidation tests type assignability validation for BindInstance
func TestBindInstance_AssignabilityValidation(t *testing.T) {
	t.Run("should allow pointer to struct assignable to interface", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			BindInstance[v2TestInterface](injector, &v2TestImpl1{value: "ok"})
		})
	})

	t.Run("should allow matching types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			BindInstance[string](injector, "test string")
		})
	})

	t.Run("should allow pointer types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			str := "test"
			BindInstance[*string](injector, &str)
		})
	})

	t.Run("should allow int types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			BindInstance[int](injector, 42)
		})
	})
}

// TestBindProvider_BasicBinding tests the basic functionality of BindProvider[T]
func TestBindProvider_BasicBinding(t *testing.T) {
	t.Run("should bind provider function", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() v2TestInterface {
			return &v2TestImpl1{value: "provided"}
		}

		binding := BindProvider[v2TestInterface](injector, provider)

		assert.NotNil(t, binding)
		assert.NotNil(t, binding.provider)
		assert.Equal(t, reflect.TypeOf(provider).Out(0), binding.provider.fnctype)
	})

	t.Run("should bind provider with dependencies", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindInstance[string](injector, "dependency")

		provider := func(s string) v2TestInterface {
			return &v2TestImpl1{value: s}
		}

		binding := BindProvider[v2TestInterface](injector, provider)

		assert.NotNil(t, binding)
		assert.NotNil(t, binding.provider)
	})

	t.Run("should bind provider returning concrete type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() *v2TestImpl1 {
			return &v2TestImpl1{value: "concrete"}
		}

		binding := BindProvider[v2TestInterface](injector, provider)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf((*v2TestImpl1)(nil)), binding.provider.fnctype)
	})

	t.Run("should add binding to injector", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() v2TestInterface {
			return &v2TestImpl1{value: "test"}
		}

		BindProvider[v2TestInterface](injector, provider)

		typ := reflect.TypeOf((*v2TestInterface)(nil)).Elem()
		assert.Contains(t, injector.bindings, typ)
		assert.Len(t, injector.bindings[typ], 1)
	})
}

// TestBindProvider_PointerHandling tests pointer handling in BindProvider[T]
func TestBindProvider_PointerHandling(t *testing.T) {
	t.Run("should strip pointer from T type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() v2TestInterface {
			return &v2TestImpl1{value: "test"}
		}

		binding := BindProvider[*v2TestInterface](injector, provider)

		assert.Equal(t, reflect.TypeOf((*v2TestInterface)(nil)).Elem(), binding.typeof)
	})

	t.Run("should handle provider returning pointer", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() *v2TestImpl1 {
			return &v2TestImpl1{value: "test"}
		}

		binding := BindProvider[v2TestInterface](injector, provider)

		assert.Equal(t, reflect.TypeOf((*v2TestImpl1)(nil)), binding.provider.fnctype)
	})

	t.Run("should handle provider returning value for concrete types", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() string {
			return "test value"
		}

		binding := BindProvider[string](injector, provider)

		assert.Equal(t, reflect.TypeOf(""), binding.provider.fnctype)
	})
}

// TestBindProvider_AssignabilityValidation tests type assignability validation for BindProvider
func TestBindProvider_AssignabilityValidation(t *testing.T) {
	t.Run("should panic when provider return type is not assignable to T", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() string {
			return "invalid"
		}

		assert.Panics(t, func() {
			BindProvider[v2TestInterface](injector, provider)
		})
	})

	t.Run("should panic with correct error message", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() int {
			return 42
		}

		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, r, "not assignable to")
			}
		}()

		BindProvider[v2TestInterface](injector, provider)
	})

	t.Run("should allow provider returning compatible type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			provider := func() *v2TestImpl1 {
				return &v2TestImpl1{value: "ok"}
			}
			BindProvider[v2TestInterface](injector, provider)
		})
	})

	t.Run("should allow provider returning exact type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			provider := func() v2TestInterface {
				return &v2TestImpl1{value: "ok"}
			}
			BindProvider[v2TestInterface](injector, provider)
		})
	})
}

// TestBindMulti_BasicBinding tests the basic functionality of BindMulti[T, U]
func TestBindMulti_BasicBinding(t *testing.T) {
	t.Run("should bind to multibindings", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := BindMulti[v2MultiBindInterface, v2MultiImpl1](injector)

		assert.NotNil(t, binding)
		assert.Equal(t, reflect.TypeOf((*v2MultiBindInterface)(nil)).Elem(), binding.typeof)
		assert.Equal(t, reflect.TypeOf(v2MultiImpl1{}), binding.to)
	})

	t.Run("should add binding to multibindings map", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl1](injector)

		typ := reflect.TypeOf((*v2MultiBindInterface)(nil)).Elem()
		assert.Contains(t, injector.multibindings, typ)
		assert.Len(t, injector.multibindings[typ], 1)
	})

	t.Run("should allow multiple multibindings of same type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl1](injector)
		BindMulti[v2MultiBindInterface, v2MultiImpl2](injector)
		BindMulti[v2MultiBindInterface, v2MultiImpl3](injector)

		typ := reflect.TypeOf((*v2MultiBindInterface)(nil)).Elem()
		assert.Len(t, injector.multibindings[typ], 3)
	})

	t.Run("should not add to regular bindings", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl1](injector)

		typ := reflect.TypeOf((*v2MultiBindInterface)(nil)).Elem()
		// Should not be in regular bindings
		assert.NotContains(t, injector.bindings, typ)
	})
}

// TestBindMulti_PointerHandling tests pointer handling in BindMulti[T, U]
func TestBindMulti_PointerHandling(t *testing.T) {
	t.Run("should strip pointer from T type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := BindMulti[*v2MultiBindInterface, v2MultiImpl1](injector)

		assert.Equal(t, reflect.TypeOf((*v2MultiBindInterface)(nil)).Elem(), binding.typeof)
	})

	t.Run("should strip pointer from U type", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := BindMulti[v2MultiBindInterface, *v2MultiImpl1](injector)

		assert.Equal(t, reflect.TypeOf(v2MultiImpl1{}), binding.to)
	})

	t.Run("should strip multiple levels of pointers from U", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		type PtrPtr **v2MultiImpl1
		binding := BindMulti[v2MultiBindInterface, PtrPtr](injector)

		assert.Equal(t, reflect.TypeOf(v2MultiImpl1{}), binding.to)
	})
}

// TestBindMulti_AssignabilityValidation tests type assignability validation for BindMulti
func TestBindMulti_AssignabilityValidation(t *testing.T) {
	t.Run("should panic when U is not assignable to T", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.Panics(t, func() {
			BindMulti[v2MultiBindInterface, int](injector)
		})
	})

	t.Run("should panic with correct error message", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, r, "not assignable to")
			}
		}()

		BindMulti[v2MultiBindInterface, string](injector)
	})

	t.Run("should allow pointer to struct assignable to interface", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		assert.NotPanics(t, func() {
			BindMulti[v2MultiBindInterface, *v2MultiImpl1](injector)
		})
	})
}

// TestBindMulti_ChainableMethods tests that multibinding methods can be chained
func TestBindMulti_ChainableMethods(t *testing.T) {
	t.Run("should support method chaining with AnnotatedWith", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := BindMulti[v2MultiBindInterface, v2MultiImpl1](injector).AnnotatedWith("multi")

		assert.Equal(t, "multi", binding.annotatedWith)
	})

	t.Run("should support method chaining with In", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		binding := BindMulti[v2MultiBindInterface, v2MultiImpl1](injector).In(Singleton)

		assert.Equal(t, Singleton, binding.scope)
	})
}

// Integration tests with the DI system

// TestBind_Integration tests Bind[T, U] integration with the DI system
func TestBind_Integration(t *testing.T) {
	t.Run("should resolve binding via GetInstance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector)

		instance, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.NotNil(t, instance)

		iface, ok := instance.(v2TestInterface)
		assert.True(t, ok)
		assert.NotNil(t, iface.Method())
	})

	t.Run("should resolve binding via generic GetInstance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector)

		instance, err := GetInstance[v2TestInterface](injector)
		assert.NoError(t, err)
		assert.NotNil(t, instance)
		assert.NotNil(t, instance.Method())
	})

	t.Run("should inject into struct fields", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector)

		type testStruct struct {
			Iface v2TestInterface `inject:""`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.NotNil(t, ts.Iface)
		assert.NotNil(t, ts.Iface.Method())
	})

	t.Run("should resolve annotated binding", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector).AnnotatedWith("impl1")
		Bind[v2TestInterface, v2TestImpl2](injector).AnnotatedWith("impl2")

		instance1, err := injector.GetAnnotatedInstance((*v2TestInterface)(nil), "impl1")
		assert.NoError(t, err)
		assert.NotNil(t, instance1)

		instance2, err := injector.GetAnnotatedInstance((*v2TestInterface)(nil), "impl2")
		assert.NoError(t, err)
		assert.NotNil(t, instance2)

		assert.Equal(t, "", instance1.(v2TestInterface).Method())
		assert.Equal(t, "impl2", instance2.(v2TestInterface).Method())
	})

	t.Run("should resolve singleton scope", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		Bind[v2TestInterface, v2TestImpl1](injector).In(Singleton)

		instance1, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)

		instance2, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)

		// Both should be the same instance
		assert.Same(t, instance1, instance2)
	})
}

// TestBindFor_Integration tests BindFor[T] integration with the DI system
func TestBindFor_Integration(t *testing.T) {
	t.Run("should resolve binding via GetInstance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		// BindFor binds to the TYPE, not the instance value
		BindFor[v2TestInterface](injector, &v2TestImpl1{value: "ignored"})

		instance, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.NotNil(t, instance)

		iface := instance.(v2TestInterface)
		// The instance will be a new one, not the one we passed
		assert.NotNil(t, iface.Method())
	})

	t.Run("should resolve with annotations", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindFor[v2TestInterface](injector, &v2TestImpl2{number: 123}).AnnotatedWith("test")

		instance, err := injector.GetAnnotatedInstance((*v2TestInterface)(nil), "test")
		assert.NoError(t, err)

		iface := instance.(v2TestInterface)
		assert.Equal(t, "impl2", iface.Method())
	})

	t.Run("should determine binding type from value", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		// Pass impl1 but bind to interface
		BindFor[v2TestInterface](injector, &v2TestImpl1{value: "test"})

		// Should create a new v2TestImpl1
		instance, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.IsType(t, &v2TestImpl1{}, instance)
	})
}

// TestBindInstance_Integration tests BindInstance[T] integration with the DI system
func TestBindInstance_Integration(t *testing.T) {
	t.Run("should resolve instance binding", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		sharedInstance := &v2TestImpl1{value: "singleton instance"}
		BindInstance[v2TestInterface](injector, sharedInstance)

		instance1, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)

		instance2, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)

		// Both should be the exact same instance
		assert.Same(t, instance1, instance2)
		assert.Same(t, sharedInstance, instance1)
	})

	t.Run("should resolve string instance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindInstance[string](injector, "test string value")

		instance, err := injector.GetInstance((*string)(nil))
		assert.NoError(t, err)

		str := instance.(string)
		assert.Equal(t, "test string value", str)
	})

	t.Run("should inject instance into struct", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindInstance[string](injector, "injected value")

		type testStruct struct {
			Str string `inject:""`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.Equal(t, "injected value", ts.Str)
	})

	t.Run("should resolve annotated instance", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindInstance[string](injector, "config value").AnnotatedWith("config")

		instance, err := injector.GetAnnotatedInstance((*string)(nil), "config")
		assert.NoError(t, err)

		str := instance.(string)
		assert.Equal(t, "config value", str)
	})
}

// TestBindProvider_Integration tests BindProvider[T] integration with the DI system
func TestBindProvider_Integration(t *testing.T) {
	t.Run("should resolve provider binding", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		counter := 0
		provider := func() v2TestInterface {
			counter++
			return &v2TestImpl1{value: "provided"}
		}

		BindProvider[v2TestInterface](injector, provider)

		instance1, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.NotNil(t, instance1)

		instance2, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.NotNil(t, instance2)

		// Provider should be called twice (not singleton)
		assert.Equal(t, 2, counter)
	})

	t.Run("should resolve provider with dependencies", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindInstance[string](injector, "dependency value")

		provider := func(s string) v2TestInterface {
			return &v2TestImpl1{value: s}
		}

		BindProvider[v2TestInterface](injector, provider)

		instance, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)

		iface := instance.(v2TestInterface)
		assert.Equal(t, "dependency value", iface.Method())
	})

	t.Run("should inject provider function", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		counter := 0
		provider := func() v2TestInterface {
			counter++
			return &v2TestImpl1{value: "from provider"}
		}

		BindProvider[v2TestInterface](injector, provider)

		type testStruct struct {
			Provider v2TestProvider `inject:""`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.NotNil(t, ts.Provider)

		result1 := ts.Provider()
		assert.Equal(t, "from provider", result1.Method())
		assert.Equal(t, 1, counter)

		result2 := ts.Provider()
		assert.Equal(t, "from provider", result2.Method())
		assert.Equal(t, 2, counter)
	})

	t.Run("should resolve provider in singleton scope", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		provider := func() v2TestInterface {
			return &v2TestImpl1{value: "singleton provider"}
		}

		BindProvider[v2TestInterface](injector, provider).In(Singleton)

		instance1, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.NotNil(t, instance1)

		instance2, err := injector.GetInstance((*v2TestInterface)(nil))
		assert.NoError(t, err)
		assert.NotNil(t, instance2)

		// Both instances should be the exact same object due to singleton scope
		assert.Same(t, instance1, instance2, "Should return the same instance for singleton")
	})
}

// TestBindMulti_Integration tests BindMulti[T, U] integration with the DI system
func TestBindMulti_Integration(t *testing.T) {
	t.Run("should resolve multiple bindings as slice", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl1](injector)
		BindMulti[v2MultiBindInterface, v2MultiImpl2](injector)
		BindMulti[v2MultiBindInterface, v2MultiImpl3](injector)

		type testStruct struct {
			Multi []v2MultiBindInterface `inject:""`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.Len(t, ts.Multi, 3)
		assert.Equal(t, "multi1", ts.Multi[0].GetValue())
		assert.Equal(t, "multi2", ts.Multi[1].GetValue())
		assert.Equal(t, "multi3", ts.Multi[2].GetValue())
	})

	t.Run("should resolve empty slice when no bindings", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		type testStruct struct {
			Multi []v2MultiBindInterface `inject:""`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.NotNil(t, ts.Multi)
		assert.Len(t, ts.Multi, 0)
	})

	t.Run("should resolve multibindings with annotations", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl1](injector).AnnotatedWith("group1")
		BindMulti[v2MultiBindInterface, v2MultiImpl2](injector).AnnotatedWith("group1")
		BindMulti[v2MultiBindInterface, v2MultiImpl3](injector).AnnotatedWith("group2")

		type testStruct struct {
			Group1 []v2MultiBindInterface `inject:"group1"`
			Group2 []v2MultiBindInterface `inject:"group2"`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.Len(t, ts.Group1, 2)
		assert.Len(t, ts.Group2, 1)
	})

	t.Run("should inherit multibindings from parent injector", func(t *testing.T) {
		parent, err := NewInjector()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl1](parent)
		BindMulti[v2MultiBindInterface, v2MultiImpl2](parent)

		child, err := parent.Child()
		assert.NoError(t, err)

		BindMulti[v2MultiBindInterface, v2MultiImpl3](child)

		type testStruct struct {
			Multi []v2MultiBindInterface `inject:""`
		}

		instance, err := child.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.Len(t, ts.Multi, 3)
	})
}

// TestMixedBindingV1AndV2 tests that v1 and v2 binding APIs can work together
func TestMixedBindingV1AndV2(t *testing.T) {
	t.Run("should resolve v1 and v2 bindings together", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		// V1 binding
		injector.Bind((*v2TestInterface)(nil)).AnnotatedWith("v1").To(v2TestImpl1{})

		// V2 binding
		Bind[v2TestInterface, v2TestImpl2](injector).AnnotatedWith("v2")

		type testStruct struct {
			V1 v2TestInterface `inject:"v1"`
			V2 v2TestInterface `inject:"v2"`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.NotNil(t, ts.V1)
		assert.NotNil(t, ts.V2)
		assert.Equal(t, "", ts.V1.Method())
		assert.Equal(t, "impl2", ts.V2.Method())
	})

	t.Run("should resolve v1 and v2 instance bindings", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		// V1 instance
		injector.Bind((*string)(nil)).AnnotatedWith("v1").ToInstance("v1 string")

		// V2 instance
		BindInstance[string](injector, "v2 string").AnnotatedWith("v2")

		type testStruct struct {
			V1Str string `inject:"v1"`
			V2Str string `inject:"v2"`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.Equal(t, "v1 string", ts.V1Str)
		assert.Equal(t, "v2 string", ts.V2Str)
	})

	t.Run("should resolve v1 and v2 multibindings together", func(t *testing.T) {
		injector, err := NewInjector()
		assert.NoError(t, err)

		// V1 multibinding
		injector.BindMulti((*v2MultiBindInterface)(nil)).To(v2MultiImpl1{})

		// V2 multibindings
		BindMulti[v2MultiBindInterface, v2MultiImpl2](injector)
		BindMulti[v2MultiBindInterface, v2MultiImpl3](injector)

		type testStruct struct {
			Multi []v2MultiBindInterface `inject:""`
		}

		instance, err := injector.GetInstance(&testStruct{})
		assert.NoError(t, err)

		ts := instance.(*testStruct)
		assert.Len(t, ts.Multi, 3)
	})
}

// Benchmark tests

func BenchmarkBind(b *testing.B) {
	injector, _ := NewInjector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Bind[v2TestInterface, v2TestImpl1](injector)
	}
}

func BenchmarkBindFor(b *testing.B) {
	injector, _ := NewInjector()
	impl := &v2TestImpl1{value: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BindFor[v2TestInterface](injector, impl)
	}
}

func BenchmarkBindInstance(b *testing.B) {
	injector, _ := NewInjector()
	impl := &v2TestImpl1{value: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BindInstance[v2TestInterface](injector, impl)
	}
}

func BenchmarkBindProvider(b *testing.B) {
	injector, _ := NewInjector()
	provider := func() v2TestInterface {
		return &v2TestImpl1{value: "test"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BindProvider[v2TestInterface](injector, provider)
	}
}

func BenchmarkBindMulti(b *testing.B) {
	injector, _ := NewInjector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BindMulti[v2MultiBindInterface, v2MultiImpl1](injector)
	}
}

func BenchmarkBindResolution(b *testing.B) {
	injector, _ := NewInjector()
	Bind[v2TestInterface, v2TestImpl1](injector)
	_ = injector.InitModules()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetInstance[v2TestInterface](injector)
	}
}
