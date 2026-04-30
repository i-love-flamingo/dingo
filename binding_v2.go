package dingo

import (
	"fmt"
	"reflect"
)

// Binding options

// BindingAttributes holds the resolved options that have been applied to a binding.
type BindingAttributes struct {
	annotation string
	Scope      Scope
	eager      bool
}

// BindingOption is a functional option that configures a BindingAttributes.
// Use Annotated, Scoped, ScopedSingleton, ScopedChildSingleton, or EagerSingleton
// to create BindingOptions and pass them to any Bind*, Swap*, MultiBind*, or MapBind* function.
type BindingOption func(*BindingAttributes)

// Annotated returns a BindingOption that attaches an annotation to a binding,
// allowing multiple bindings of the same type to coexist under different names.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).AnnotatedWith("myAnnotation")
func Annotated(text string) BindingOption {
	return func(attributes *BindingAttributes) {
		attributes.annotation = text
	}
}

// Scoped returns a BindingOption that sets the scope for a binding,
// for example placing it in a singleton scope so only one instance is created.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).In(dingo.Singleton)
func Scoped(scope Scope) BindingOption {
	return func(attributes *BindingAttributes) {
		attributes.Scope = scope
	}
}

// ScopedSingleton returns a BindingOption that places the binding in the
// Singleton scope: only one instance is ever created for the whole injector.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).In(dingo.Singleton)
func ScopedSingleton() BindingOption {
	return Scoped(Singleton)
}

// ScopedChildSingleton returns a BindingOption that places the binding in the
// ChildSingleton scope: one instance per child injector is created.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).In(dingo.ChildSingleton)
func ScopedChildSingleton() BindingOption {
	return Scoped(ChildSingleton)
}

// EagerSingleton returns a BindingOption that places the binding in the
// Singleton scope and, when value is true, requests eager initialization:
// the instance is created immediately when the injector is built rather than
// on first use. Passing false keeps the singleton scope but disables eager
// initialization, behaving like ScopedSingleton.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).To(MyImpl{}).AsEagerSingleton()
func EagerSingleton(value bool) BindingOption {
	return func(attributes *BindingAttributes) {
		attributes.Scope = Singleton
		attributes.eager = value
	}
}

// Bindings

// BindTransient registers a transient binding: each time T is resolved, a fresh
// instance of the concrete type `what` is created and injected.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).To(new(MyImpl))
func BindTransient[T any](injector *Injector, what T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.Bind(target).
		To(what).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// BindValue registers a fixed-instance binding: every resolution of T returns
// exactly the same instance that was passed in.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).ToInstance(myInstance)
func BindValue[T any](injector *Injector, instance T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.Bind(target).
		ToInstance(instance).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// BindProvider registers a provider function for T. The provider's arguments are
// automatically injected on each call. Returns an error if the provider signature
// is incompatible with T.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).ToProvider(myProviderFunc)
func BindProvider[T any](injector *Injector, provider any, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.Bind(target).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if err := bindToProvider(binding, provider); err != nil {
		return nil, err
	}

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MustBindTransient is like BindTransient but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).To(new(MyImpl))
func MustBindTransient[T any](injector *Injector, what T, opts ...BindingOption) *Binding {
	binding, err := BindTransient[T](injector, what, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustBindValue is like BindValue but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).ToInstance(myInstance)
func MustBindValue[T any](injector *Injector, instance T, opts ...BindingOption) *Binding {
	binding, err := BindValue[T](injector, instance, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustBindProvider is like BindProvider but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.Bind((*MyInterface)(nil)).ToProvider(myProviderFunc)
func MustBindProvider[T any](injector *Injector, provider any, opts ...BindingOption) *Binding {
	binding, err := BindProvider[T](injector, provider, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// Overrides

// SwapTransient overrides an existing binding for T with a new transient binding
// that resolves to a fresh instance of `what` on every resolution. The override is
// applied after all modules are loaded.
//
// Old API equivalent:
//
//	injector.Override((*MyInterface)(nil), "").To(new(MyImpl))
func SwapTransient[T any](injector *Injector, what T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.
		Override(target, attrs.annotation).
		AnnotatedWith(attrs.annotation).
		To(what).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// SwapValue overrides an existing binding for T with the given fixed instance.
// Every resolution after the override returns that same instance.
//
// Old API equivalent:
//
//	injector.Override((*MyInterface)(nil), "").ToInstance(myInstance)
func SwapValue[T any](injector *Injector, instance T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.
		Override(target, attrs.annotation).
		AnnotatedWith(attrs.annotation).
		ToInstance(instance).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// SwapProvider overrides an existing binding for T with a provider function.
// The provider's arguments are automatically injected on each call.
// Returns an error if the provider signature is incompatible with T.
//
// Old API equivalent:
//
//	injector.Override((*MyInterface)(nil), "").ToProvider(myProviderFunc)
func SwapProvider[T any](injector *Injector, provider any, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.
		Override(target, attrs.annotation).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if err := bindToProvider(binding, provider); err != nil {
		return nil, err
	}

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MustSwapTransient is like SwapTransient but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.Override((*MyInterface)(nil), "").To(new(MyImpl))
func MustSwapTransient[T any](injector *Injector, what T, opts ...BindingOption) *Binding {
	binding, err := SwapTransient[T](injector, what, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustSwapValue is like SwapValue but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.Override((*MyInterface)(nil), "").ToInstance(myInstance)
func MustSwapValue[T any](injector *Injector, instance T, opts ...BindingOption) *Binding {
	binding, err := SwapValue[T](injector, instance, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustSwapProvider is like SwapProvider but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.Override((*MyInterface)(nil), "").ToProvider(myProviderFunc)
func MustSwapProvider[T any](injector *Injector, provider any, opts ...BindingOption) *Binding {
	binding, err := SwapProvider[T](injector, provider, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// Multibindings

// MultiBindTransient adds a transient entry to the multibinding slice for T.
// Each resolution of []T appends a fresh instance of `what` to the result slice.
//
// Old API equivalent:
//
//	injector.BindMulti((*MyInterface)(nil)).To(new(MyImpl))
func MultiBindTransient[T any](injector *Injector, what T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.BindMulti(target).
		To(what).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MultiBindValue adds a fixed instance entry to the multibinding slice for T.
// Every resolution of []T includes the same instance in the result slice.
//
// Old API equivalent:
//
//	injector.BindMulti((*MyInterface)(nil)).ToInstance(myInstance)
func MultiBindValue[T any](injector *Injector, instance T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.BindMulti(target).
		ToInstance(instance).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MultiBindProvider adds a provider-backed entry to the multibinding slice for T.
// The provider's arguments are automatically injected on each call.
// Returns an error if the provider signature is incompatible with T.
//
// Old API equivalent:
//
//	injector.BindMulti((*MyInterface)(nil)).ToProvider(myProviderFunc)
func MultiBindProvider[T any](injector *Injector, provider any, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.
		BindMulti(target).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if err := bindToProvider(binding, provider); err != nil {
		return nil, err
	}

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MustMultiBindTransient is like MultiBindTransient but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindMulti((*MyInterface)(nil)).To(new(MyImpl))
func MustMultiBindTransient[T any](injector *Injector, what T, opts ...BindingOption) *Binding {
	binding, err := MultiBindTransient[T](injector, what, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustMultiBindValue is like MultiBindValue but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindMulti((*MyInterface)(nil)).ToInstance(myInstance)
func MustMultiBindValue[T any](injector *Injector, instance T, opts ...BindingOption) *Binding {
	binding, err := MultiBindValue[T](injector, instance, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustMultiBindProvider is like MultiBindProvider but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindMulti((*MyInterface)(nil)).ToProvider(myProviderFunc)
func MustMultiBindProvider[T any](injector *Injector, provider any, opts ...BindingOption) *Binding {
	binding, err := MultiBindProvider[T](injector, provider, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// Map bindings

// MapBindTransient adds a transient entry under key to the map binding for T.
// Each resolution of map[string]T for that key returns a fresh instance of `what`.
//
// Old API equivalent:
//
//	injector.BindMap((*MyInterface)(nil), "myKey").To(new(MyImpl))
func MapBindTransient[T any](injector *Injector, key string, what T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.
		BindMap(target, key).
		To(what).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MapBindValue adds a fixed-instance entry under key to the map binding for T.
// Every resolution of map[string]T for that key returns the same instance.
//
// Old API equivalent:
//
//	injector.BindMap((*MyInterface)(nil), "myKey").ToInstance(myInstance)
func MapBindValue[T any](injector *Injector, key string, instance T, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.BindMap(target, key).
		ToInstance(instance).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MapBindProvider adds a provider-backed entry under key to the map binding for T.
// The provider's arguments are automatically injected on each call.
// Returns an error if the provider signature is incompatible with T.
//
// Old API equivalent:
//
//	injector.BindMap((*MyInterface)(nil), "myKey").ToProvider(myProviderFunc)
func MapBindProvider[T any](injector *Injector, key string, provider any, opts ...BindingOption) (*Binding, error) {
	attrs := applyOpts(opts...)

	var target *T

	binding := injector.BindMap(target, key).
		AnnotatedWith(attrs.annotation).
		In(attrs.Scope)

	if err := bindToProvider(binding, provider); err != nil {
		return nil, err
	}

	if attrs.eager {
		binding = binding.AsEagerSingleton()
	}

	return binding, nil
}

// MustMapBindTransient is like MapBindTransient but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindMap((*MyInterface)(nil), "myKey").To(new(MyImpl))
func MustMapBindTransient[T any](injector *Injector, key string, what T, opts ...BindingOption) *Binding {
	binding, err := MapBindTransient[T](injector, key, what, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustMapBindValue is like MapBindValue but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindMap((*MyInterface)(nil), "myKey").ToInstance(myInstance)
func MustMapBindValue[T any](injector *Injector, key string, instance T, opts ...BindingOption) *Binding {
	binding, err := MapBindValue[T](injector, key, instance, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// MustMapBindProvider is like MapBindProvider but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindMap((*MyInterface)(nil), "myKey").ToProvider(myProviderFunc)
func MustMapBindProvider[T any](injector *Injector, key string, provider any, opts ...BindingOption) *Binding {
	binding, err := MapBindProvider[T](injector, key, provider, opts...)
	if err != nil {
		panic(err)
	}

	return binding
}

// Intercept

// Intercept registers interceptor as an AOP-style wrapper for the interface T.
// T must be an interface. Multiple interceptors are applied in registration order.
// Returns an error if T is not an interface, if interceptor is nil, or if the
// interceptor's concrete type does not implement T.
//
// Old API equivalent:
//
//	injector.BindInterceptor((*MyInterface)(nil), new(MyInterceptor))
func Intercept[T any](injector *Injector, interceptor T) error {
	totype := reflect.TypeFor[T]()
	if totype.Kind() == reflect.Ptr {
		totype = totype.Elem()
	}

	if totype.Kind() != reflect.Interface {
		return fmt.Errorf("%w: can only intercept interfaces, got %s", ErrIncorrectBinding, totype)
	}

	interceptorType := reflect.TypeOf(interceptor)
	if interceptorType == nil {
		return fmt.Errorf("%w: interceptor must not be nil for interface %s", ErrIncorrectBinding, totype)
	}

	if interceptorType.Kind() == reflect.Ptr {
		interceptorType = interceptorType.Elem()
	}

	// Validate that the concrete interceptor type (or its pointer) implements T.
	concretePtr := reflect.PointerTo(interceptorType)
	if !concretePtr.Implements(totype) && !interceptorType.Implements(totype) {
		return fmt.Errorf("%w: interceptor %s does not implement %s", ErrIncorrectBinding, interceptorType, totype)
	}

	m := injector.interceptor[totype]
	m = append(m, interceptorType)
	injector.interceptor[totype] = m

	return nil
}

// MustIntercept is like Intercept but panics instead of returning an error.
//
// Old API equivalent:
//
//	injector.BindInterceptor((*MyInterface)(nil), new(MyInterceptor))
func MustIntercept[T any](injector *Injector, interceptor T) {
	err := Intercept[T](injector, interceptor)
	if err != nil {
		panic(err)
	}
}

// Resolution

// Get resolves T from the injector and writes the result into result.
// It is the generic counterpart of injector.GetInstance.
//
// Old API equivalent:
//
//	instance, err := injector.GetInstance((*MyType)(nil))
func Get[T any](injector *Injector, result *T) error {
	instance, err := injector.GetInstance(reflect.TypeFor[T]())
	if err != nil {
		return err
	}

	v, ok := instance.(T)
	if !ok {
		return fmt.Errorf("%w: resolved value of type %T is not assignable to %T", ErrIncorrectBinding, instance, *result)
	}

	*result = v

	return nil
}

// GetAnnotated resolves an annotated binding of T from the injector and writes
// the result into result. It is the generic counterpart of
// injector.GetAnnotatedInstance.
//
// Old API equivalent:
//
//	instance, err := injector.GetAnnotatedInstance((*MyType)(nil), "myAnnotation")
func GetAnnotated[T any](injector *Injector, annotation string, result *T) error {
	instance, err := injector.GetAnnotatedInstance(reflect.TypeFor[T](), annotation)
	if err != nil {
		return err
	}

	v, ok := instance.(T)
	if !ok {
		return fmt.Errorf("%w: resolved value of type %T is not assignable to %T", ErrIncorrectBinding, instance, *result)
	}

	*result = v

	return nil
}

// MustGet is like Get but panics instead of returning an error.
//
// Old API equivalent:
//
//	instance, err := injector.GetInstance((*MyType)(nil))
func MustGet[T any](injector *Injector, result *T) {
	if err := Get[T](injector, result); err != nil {
		panic(err)
	}
}

// MustGetAnnotated is like GetAnnotated but panics instead of returning an error.
//
// Old API equivalent:
//
//	instance, err := injector.GetAnnotatedInstance((*MyType)(nil), "myAnnotation")
func MustGetAnnotated[T any](injector *Injector, annotation string, result *T) {
	if err := GetAnnotated[T](injector, annotation, result); err != nil {
		panic(err)
	}
}

// Internal helpers

func applyOpts(opts ...BindingOption) BindingAttributes {
	var attrs BindingAttributes
	for _, opt := range opts {
		opt(&attrs)
	}

	return attrs
}
