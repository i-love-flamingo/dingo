package dingo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	tryModuleOk    struct{}
	tryModuleFail  struct{}
	tryModulePanic struct{}
)

func (t *tryModuleOk) Configure(injector *Injector) {
	injector.Bind(new(string)).ToInstance("test")
}

func (t *tryModuleFail) Configure(injector *Injector) {
	injector.Bind(new(int)).ToInstance("test")
}

func (t *tryModulePanic) Configure(injector *Injector) {
	injector.Bind(nil)
}

func TestTryModule(t *testing.T) {
	assert.NoError(t, TryModule(new(tryModuleOk)))

	assert.Error(t, TryModule(new(tryModuleFail)))

	assert.Error(t, TryModule(new(tryModulePanic)))
}

type (
	resolveDependenciesModuleA  struct{}
	resolveDependenciesModuleB  struct{}
	resolveDependenciesModuleB2 struct{}
	resolveDependenciesModuleC  struct{}
)

func (*resolveDependenciesModuleA) Configure(*Injector) {}
func (*resolveDependenciesModuleA) Depends() []Module {
	return []Module{
		new(resolveDependenciesModuleA),
		new(resolveDependenciesModuleB),
		new(resolveDependenciesModuleB2),
	}
}
func (*resolveDependenciesModuleB) Configure(*Injector) {}
func (*resolveDependenciesModuleB) Depends() []Module {
	return []Module{
		new(resolveDependenciesModuleC),
		new(resolveDependenciesModuleB2),
	}
}
func (*resolveDependenciesModuleB2) Configure(*Injector) {}
func (*resolveDependenciesModuleC) Configure(*Injector)  {}

func Test_resolveDependencies(t *testing.T) {
	resolved := resolveDependencies([]Module{new(resolveDependenciesModuleA)}, nil)

	if !reflect.DeepEqual(resolved, []Module{
		new(resolveDependenciesModuleC),
		new(resolveDependenciesModuleB2),
		new(resolveDependenciesModuleB),
		new(resolveDependenciesModuleA),
	}) {
		t.Errorf("%#v not correctly resolved", resolved)
	}
}
