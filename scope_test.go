package dingo

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testScope(t *testing.T, scope Scope) {
	var requestedUnscoped int64

	test := reflect.TypeOf("string")
	test2 := reflect.TypeOf("int")

	unscoped := func(t reflect.Type, annotation string, optional bool) (reflect.Value, error) {
		atomic.AddInt64(&requestedUnscoped, 1)

		time.Sleep(1 * time.Nanosecond)

		if optional {
			return reflect.Value{}, nil
		}
		return reflect.New(t).Elem(), nil
	}

	runs := 100 // change to 10? 100? 1000? to trigger a bug? todo investigate

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
			assert.Equal(t, t1, t2)
			assert.Equal(t, t12, t22)
			wg.Done()
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), requestedUnscoped)

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
		C singletonC `inject:""`
	}

	singletonC string
)

func TestScopeWithSubDependencies(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("Run %d", i), func(t *testing.T) {
			injector, err := NewInjector()
			assert.NoError(t, err)

			injector.Bind(new(singletonA)).In(Singleton)
			injector.Bind(new(singletonB)).In(Singleton)
			injector.Bind(singletonC("")).In(Singleton).ToInstance(singletonC("singleton C"))

			runs := 10

			wg := new(sync.WaitGroup)
			wg.Add(runs)
			for i := 0; i < runs; i++ {
				go func() {
					i, err := injector.GetInstance(new(singletonA))
					assert.NoError(t, err)
					a := i.(*singletonA)
					assert.Equal(t, a.B.C, singletonC("singleton C"))
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}
