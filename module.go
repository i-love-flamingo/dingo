package dingo

import (
	"errors"
	"fmt"
	"reflect"
)

type (
	// Module is provided by packages to generate the DI tree
	Module interface {
		Configure(injector *Injector)
	}

	// Depender defines a dependency-aware module
	Depender interface {
		Depends() []Module
	}
)

// TryModule tests if modules are properly bound
func TryModule(modules ...Module) (resultingError error) {
	defer func() {
		if err := recover(); err != nil {
			if err, ok := err.(error); ok {
				resultingError = err
				return
			}
			resultingError = errors.New(fmt.Sprint(err))
		}
	}()

	injector, err := NewInjector()
	if err != nil {
		return err
	}
	injector.buildEagerSingletons = false
	return injector.InitModules(modules...)
}

// resolveDependencies tries to get a complete list of all modules, including all dependencies
// known can be empty initially, and will then be used for subsequent recursive calls
func resolveDependencies(modules []Module, known map[reflect.Type]struct{}) []Module {
	final := make([]Module, 0, len(modules))

	if known == nil {
		known = make(map[reflect.Type]struct{})
	}

	for _, module := range modules {
		if _, ok := known[reflect.TypeOf(module)]; ok {
			continue
		}
		known[reflect.TypeOf(module)] = struct{}{}
		if depender, ok := module.(Depender); ok {
			final = append(final, resolveDependencies(depender.Depends(), known)...)
		}
		final = append(final, module)
	}

	return final
}
