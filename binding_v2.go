package dingo

import (
	"fmt"
	"reflect"
)

// Helper functions for generic binding operations

// stripPtrType removes one level of pointer indirection from a type.
// If the type is not a pointer, it returns the type unchanged.
func stripPtrType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		return t.Elem()
	}
	return t
}

// stripAllPtrs removes all levels of pointer indirection from a type.
// It keeps unwrapping pointers until it reaches a non-pointer type.
func stripAllPtrs(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// typeForNoPtr returns the reflect.Type for T with any pointer level stripped.
// This is a convenience wrapper around reflect.TypeFor and stripPtrType.
func typeForNoPtr[T any]() reflect.Type {
	return stripPtrType(reflect.TypeFor[T]())
}

// isAssignable checks if the 'from' type can be assigned to the 'to' type.
// It also considers the case where a pointer to 'from' is assignable to 'to'.
func isAssignable(from, to reflect.Type) bool {
	return from.AssignableTo(to) || reflect.PointerTo(from).AssignableTo(to)
}

func Bind[T, U any](injector *Injector) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := stripAllPtrs(reflect.TypeFor[U]())

	if !isAssignable(to, binding.typeof) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", to.PkgPath(), to.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	binding.to = to

	return binding
}

func BindFor[T any](injector *Injector, what T) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := stripAllPtrs(reflect.TypeOf(what))

	if !isAssignable(to, binding.typeof) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", to.PkgPath(), to.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	binding.to = to

	return binding
}

func BindInstance[T any](injector *Injector, instance T) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	binding.instance = &Instance{
		itype:  reflect.TypeOf(instance),
		ivalue: reflect.ValueOf(instance),
	}
	// For BindInstance, we check if itype is assignable to typeof or to *typeof
	// (not if *itype is assignable to typeof, which is what isAssignable does)
	if !binding.instance.itype.AssignableTo(binding.typeof) && !binding.instance.itype.AssignableTo(reflect.PointerTo(binding.typeof)) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", binding.instance.itype.PkgPath(), binding.instance.itype.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	return binding
}

func BindProvider[T any](injector *Injector, fn any) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	provider := &Provider{
		fnc:     reflect.ValueOf(fn),
		binding: binding,
	}

	provider.fnctype = provider.fnc.Type().Out(0)
	// For BindProvider, we check if fnctype is assignable to typeof or to *typeof
	// (not if *fnctype is assignable to typeof, which is what isAssignable does)
	if !provider.fnctype.AssignableTo(binding.typeof) && !provider.fnctype.AssignableTo(reflect.PointerTo(binding.typeof)) {
		panic(fmt.Sprintf("provider returns %q which is not assignable to %q", provider.fnctype, binding.typeof))
	}

	binding.provider = provider

	return binding
}

func BindMulti[T, U any](injector *Injector) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype

	to := stripAllPtrs(reflect.TypeFor[U]())

	if !isAssignable(to, binding.typeof) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", to.PkgPath(), to.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	binding.to = to

	injector.multibindings[bindtype] = append(injector.multibindings[bindtype], binding)

	return binding
}
