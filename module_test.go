package dingo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResolveDependenciesWithModuleFunc(t *testing.T) {
	var countInline, countExtern int

	ext := ModuleFunc(func(injector *Injector) {
		countExtern++
	})

	injector, err := NewInjector(
		new(tryModuleOk),
		ModuleFunc(func(injector *Injector) {
			countInline++
		}),
		ModuleFunc(func(injector *Injector) {
			countInline++
		}),
		ext,
		ext,
	)

	assert.NoError(t, err)
	assert.NotNil(t, injector)
	assert.Equal(t, 2, countInline, "inline modules should be called once (eventually twice for this test)")
	assert.Equal(t, 1, countExtern, "variable defined modules should only be called once")
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
	mg := newModuleGraph()
	err := mg.Add(new(resolveDependenciesModuleA))
	require.NoError(t, err)
	resolved, err := mg.Sort()
	require.NoError(t, err)

	assert.Equal(t, []Module{
		new(resolveDependenciesModuleB2),
		new(resolveDependenciesModuleC),
		new(resolveDependenciesModuleB),
		new(resolveDependenciesModuleA),
	}, resolved)
}

var (
	_ Module = new(A)
	_ Module = new(B)
	_ Module = new(C)
	_ Module = new(D)

	_ Depender = new(A)
	_ Depender = new(C)
	_ Depender = new(E)
)

// Module Graph
type (
	A struct {
		withCycle   bool
		SampleText1 string `inject:"test1"`
	}
	B struct{}
	C struct{ withCycle bool }
	D struct{}
	E struct {
		SampleText2 string `inject:"test2"`
	}
)

func (a *A) Configure(i *Injector) {
	i.Bind(new(string)).AnnotatedWith("test2").ToInstance("test2")
}

func (b *B) Configure(i *Injector) {
	i.Bind(new(string)).AnnotatedWith("test1").ToInstance("test1")
}

func (c *C) Configure(_ *Injector) {
}

func (d *D) Configure(_ *Injector) {
}

func (e *E) Configure(_ *Injector) {
}

func (a *A) Depends() []Module {
	return []Module{new(B), &C{withCycle: a.withCycle}}
}

func (c *C) Depends() []Module {
	deps := []Module{
		new(B),
		new(D),
	}

	if c.withCycle {
		deps = append(deps, new(E))
	}

	return deps
}

func (e *E) Depends() []Module {
	return []Module{new(A)}
}

func TestModGraph_Sorted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modules   []Module
		sorted    []Module
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "no cycle",
			modules:   []Module{&A{withCycle: false}, new(B), &C{withCycle: false}, new(D), new(E)},
			sorted:    []Module{new(B), new(D), &C{withCycle: false}, &A{withCycle: false}, new(E)},
			assertErr: assert.NoError,
		},
		{
			name:    "cycle A→C→E→A",
			modules: []Module{&A{withCycle: true}, new(B), new(D), new(E)},
			assertErr: assert.ErrorAssertionFunc(func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrModuleCycle) &&
					assert.ErrorContains(t, err, "cyclic module dependency: *dingo.A → *dingo.C → *dingo.E")
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mg := newModuleGraph()
			err := mg.Add(test.modules...)
			require.NoError(t, err)

			sorted, err := mg.Sort()
			if test.assertErr(t, err) {
				assert.Equalf(t, test.sorted, sorted, "Sorted()")
			}
		})
	}
}

func TestWithInjector(t *testing.T) {
	t.Parallel()

	injector, err := NewInjector()
	require.NoError(t, err)

	modules := []Module{new(E), new(D), new(C), new(B), new(A)}

	err = injector.InitModules(modules...)
	assert.NoError(t, err)
}
