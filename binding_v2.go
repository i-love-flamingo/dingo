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

// formatTypeName formats a reflect.Type for display in error messages.
// It handles builtin types (which have no PkgPath) and fully-qualified types.
func formatTypeName(t reflect.Type) string {
	if t.PkgPath() == "" {
		// Builtin type (string, int, etc.) or unnamed type
		return t.String()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

// formatTypeNotAssignableError creates a consistent error message for type mismatch errors.
// It includes the function context and properly formatted type names.
func formatTypeNotAssignableError(from, to reflect.Type, context string) string {
	return fmt.Sprintf("dingo: %s: type %s is not assignable to %s",
		context,
		formatTypeName(from),
		formatTypeName(to))
}

func Bind[T, U any](injector *Injector) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := stripAllPtrs(reflect.TypeFor[U]())

	if !isAssignable(to, binding.typeof) {
		panic(formatTypeNotAssignableError(to, binding.typeof, "Bind[T, U]"))
	}

	binding.to = to

	return binding
}

// BindLike binds type T to the same concrete type as the provided example value.
//
// The example value is only used to determine the concrete type to bind to.
// The value itself is NOT stored or used as an instance. If you want to bind
// a specific instance, use BindInstance instead.
//
// This is useful when you want to avoid repeating the full type name, especially
// for complex types, and let Go infer it from a value.
//
// Example:
//
//	type UserService interface {
//	    GetUser(id int) (*User, error)
//	}
//
//	type UserServiceImpl struct {
//	    db *Database
//	}
//
//	impl := &UserServiceImpl{}
//	BindLike[UserService](injector, impl)  // Binds UserService to *UserServiceImpl type
//
// This creates new instances of *UserServiceImpl when UserService is requested,
// it does NOT reuse the 'impl' variable.
func BindLike[T any](injector *Injector, example T) *Binding {
	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := stripAllPtrs(reflect.TypeOf(example))

	if !isAssignable(to, binding.typeof) {
		panic(formatTypeNotAssignableError(to, binding.typeof, "BindLike[T]"))
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
		panic(formatTypeNotAssignableError(binding.instance.itype, binding.typeof, "BindInstance[T]"))
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
		panic(formatTypeNotAssignableError(provider.fnctype, binding.typeof, "BindProvider[T]"))
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
		panic(formatTypeNotAssignableError(to, binding.typeof, "BindMulti[T, U]"))
	}

	binding.to = to

	injector.multibindings[bindtype] = append(injector.multibindings[bindtype], binding)

	return binding
}
