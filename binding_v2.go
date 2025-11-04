package dingo

import (
	"fmt"
	"reflect"
)

func Bind[T, U any](injector *Injector) *Binding {
	bindtype := reflect.TypeFor[T]()
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := reflect.TypeFor[U]()

	for to.Kind() == reflect.Ptr {
		to = to.Elem()
	}

	if !to.AssignableTo(binding.typeof) && !reflect.PointerTo(to).AssignableTo(binding.typeof) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", to.PkgPath(), to.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	binding.to = to

	return binding
}

func BindFor[T any](injector *Injector, what T) *Binding {
	bindtype := reflect.TypeFor[T]()
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := reflect.TypeOf(what)

	for to.Kind() == reflect.Ptr {
		to = to.Elem()
	}

	if !to.AssignableTo(binding.typeof) && !reflect.PointerTo(to).AssignableTo(binding.typeof) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", to.PkgPath(), to.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	binding.to = to

	return binding
}

func BindInstance[T any](injector *Injector, instance T) *Binding {
	bindtype := reflect.TypeFor[T]()
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	binding.instance = &Instance{
		itype:  reflect.TypeOf(instance),
		ivalue: reflect.ValueOf(instance),
	}
	if !binding.instance.itype.AssignableTo(binding.typeof) && !binding.instance.itype.AssignableTo(reflect.PointerTo(binding.typeof)) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", binding.instance.itype.PkgPath(), binding.instance.itype.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	return binding
}

func BindProvider[T any](injector *Injector, fn any) *Binding {
	bindtype := reflect.TypeFor[T]()
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	provider := &Provider{
		fnc:     reflect.ValueOf(fn),
		binding: binding,
	}

	provider.fnctype = provider.fnc.Type().Out(0)
	if !provider.fnctype.AssignableTo(binding.typeof) && !provider.fnctype.AssignableTo(reflect.PointerTo(binding.typeof)) {
		panic(fmt.Sprintf("provider returns %q which is not assignable to %q", provider.fnctype, binding.typeof))
	}

	binding.provider = provider

	return binding
}

func BindMulti[T, U any](injector *Injector) *Binding {
	bindtype := reflect.TypeFor[T]()
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}

	binding := new(Binding)
	binding.typeof = bindtype

	to := reflect.TypeFor[U]()

	for to.Kind() == reflect.Ptr {
		to = to.Elem()
	}

	if !to.AssignableTo(binding.typeof) && !reflect.PointerTo(to).AssignableTo(binding.typeof) {
		panic(fmt.Sprintf("%s#%s not assignable to %s#%s", to.PkgPath(), to.Name(), binding.typeof.PkgPath(), binding.typeof.Name()))
	}

	binding.to = to

	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb

	return binding
}
