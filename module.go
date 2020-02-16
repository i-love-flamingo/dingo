package dingo

import (
	"fmt"
	"reflect"
)

type (
	// Module is default entry point for dingo Modules.
	// The Configure method is called once during initialization
	// and let's the module setup Bindings for the provided Injector.
	Module interface {
		Configure(injector *Injector)
	}

	// ModuleFunc wraps a func(injector *Injector) for dependency injection.
	// This allows using small functions as dingo Modules.
	// The same concept is http.HandlerFunc for http.Handler.
	ModuleFunc func(injector *Injector)

	// Depender returns a list of Modules via the Depends method.
	// This allows a module to specify dependencies, which will be loaded before the actual Module is loaded.
	Depender interface {
		Depends() []Module
	}
)

// Configure call the original ModuleFunc with the given *Injector.
func (f ModuleFunc) Configure(injector *Injector) {
	f(injector)
}

// TryModule tests if modules are properly bound
func TryModule(modules ...Module) (resultingError error) {
	defer func() {
		if err := recover(); err != nil {
			if err, ok := err.(error); ok {
				resultingError = err
				return
			}
			resultingError = fmt.Errorf("dingo.TryModule panic: %q", err)
		}
	}()

	injector, err := NewInjector()
	if err != nil {
		return err
	}
	injector.buildEagerSingletons = false
	return injector.InitModules(modules...)
}

var typeOfModuleFunc = reflect.TypeOf(ModuleFunc(nil))

// resolveDependencies tries to get a complete list of all modules, including all dependencies
// known can be empty initially, and will then be used for subsequent recursive calls
func resolveDependencies(modules []Module, known map[interface{}]struct{}) []Module {
	final := make([]Module, 0, len(modules))

	if known == nil {
		known = make(map[interface{}]struct{})
	}

	for _, module := range modules {
		var identity interface{} = reflect.TypeOf(module)
		if identity == typeOfModuleFunc {
			identity = reflect.ValueOf(module)
		}
		if _, ok := known[identity]; ok {
			continue
		}
		known[identity] = struct{}{}
		if depender, ok := module.(Depender); ok {
			final = append(final, resolveDependencies(depender.Depends(), known)...)
		}
		final = append(final, module)
	}

	return final
}
