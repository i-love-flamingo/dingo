package dingo

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testScope(t *testing.T, scope Scope) {
	var requestedUnscoped int64

	test := reflect.TypeOf(new(string))
	test2 := reflect.TypeOf(new(int))

	unscoped := func(t reflect.Type, annotation string, optional bool) (reflect.Value, error) {
		atomic.AddInt64(&requestedUnscoped, 1)

		runtime.Gosched()

		if optional {
			return reflect.Value{}, nil
		}
		return reflect.New(t).Elem(), nil
	}

	runs := 1000 // change to 10? 100? 1000? to trigger a bug? todo investigate

	wg := new(sync.WaitGroup)
	wg.Add(runs)
	for i := 0; i < runs; i++ {
		go func() {
			t1, err := scope.ResolveType(test, "", unscoped)
			assert.NoError(t, err)
			t12, err := scope.ResolveType(test2, "", unscoped)
			assert.NoError(t, err)
			t2, err := scope.ResolveType(test, "", unscoped)
			assert.NoError(t, err)
			t22, err := scope.ResolveType(test2, "", unscoped)
			assert.NoError(t, err)
			assert.Same(t, t1.Interface(), t2.Interface())
			assert.Same(t, t12.Interface(), t22.Interface())
			wg.Done()
		}()
	}
	wg.Wait()

	// should be 2, one for each type
	assert.Equal(t, int64(2), requestedUnscoped)

}

func TestSingleton_ResolveType(t *testing.T) {
	// reset instance
	Singleton = NewSingletonScope()

	testScope(t, Singleton)
}

func TestChildSingleton_ResolveType(t *testing.T) {
	// reset instance
	ChildSingleton = NewChildSingletonScope()

	testScope(t, ChildSingleton)
}

type (
	singletonA struct {
		B *singletonB `inject:""`
	}

	singletonB struct {
		C *singletonC `inject:""`
	}

	singletonC string
)

func TestScopeWithSubDependencies(t *testing.T) {
	sc := singletonC("singleton C")
	scp := &sc
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("Run %d", i), func(t *testing.T) {
			injector, err := NewInjector()
			assert.NoError(t, err)

			injector.Bind(new(singletonA)).In(Singleton)
			injector.Bind(new(singletonB)).In(Singleton)
			injector.Bind(new(singletonC)).In(Singleton).ToInstance(scp)

			runs := 100

			wg := new(sync.WaitGroup)
			wg.Add(runs)
			for i := 0; i < runs; i++ {
				go func() {
					i, err := injector.GetInstance(new(singletonA))
					assert.NoError(t, err)
					a := i.(*singletonA)
					assert.Same(t, a.B.C, scp)
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}

type inheritedScopeIface interface{}
type inheritedScopeStruct struct{}
type inheritedScopeInjected struct {
	i inheritedScopeIface
	s *inheritedScopeStruct
}

func (s *inheritedScopeInjected) Inject(ss *inheritedScopeStruct, si inheritedScopeIface) {
	s.s = ss
	s.i = si
}

func TestInheritedScope(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.Bind(new(inheritedScopeStruct)).In(ChildSingleton)
	injector.Bind(new(inheritedScopeIface)).To(new(inheritedScopeStruct))

	injector, err = injector.Child()
	assert.NoError(t, err)

	i, err := injector.GetInstance(new(inheritedScopeInjected))
	assert.NoError(t, err)
	firstS := i.(*inheritedScopeInjected)
	i, err = injector.GetInstance(new(inheritedScopeInjected))
	assert.NoError(t, err)
	secondS := i.(*inheritedScopeInjected)
	assert.Same(t, firstS.s, secondS.s)

	i, err = injector.GetInstance(new(inheritedScopeInjected))
	assert.NoError(t, err)
	firstI := i.(*inheritedScopeInjected)
	i, err = injector.GetInstance(new(inheritedScopeInjected))
	assert.NoError(t, err)
	secondI := i.(*inheritedScopeInjected)
	assert.Same(t, firstI.i, secondI.i)
}

type circSingletonA struct {
	B *circSingletonB `inject:""`
}
type circSingletonB struct {
	A *circSingletonA `inject:""`
}

func TestCircularSingletonBinding(t *testing.T) {
	EnableCircularTracing()
	defer func() {
		traceCircular = nil
	}()
	injector, err := NewInjector()
	assert.NoError(t, err)
	injector.Bind(new(circSingletonA)).In(Singleton)
	assert.Panics(t, func() {
		injector.GetInstance(new(circSingletonA))
	})
}
