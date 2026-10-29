package dingo

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	circA struct {
		A *circA `inject:""`
		B *circB `inject:""`
	}

	circB struct {
		A *circA `inject:""`
		B *circB `inject:""`
	}

	circCProvider  func() circCInterface
	circCInterface interface{}
	circC          struct {
		C circCProvider `inject:""`
	}

	circAProvider func() *circA
	circD         struct {
		A circAProvider `inject:""`
	}
)

func TestDingoCircula(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)
	injector.EnableCircularTracing()

	assert.Panics(t, func() {
		i, err := injector.GetInstance(new(circA))
		assert.NoError(t, err)
		_, ok := i.(*circA)
		if !ok {
			t.Fail()
		}
	})

	injector.Bind(new(circCInterface)).To(circC{})

	i, err := injector.GetInstance(new(circC))
	assert.NoError(t, err)
	c, ok := i.(*circC)
	if !ok {
		t.Fail()
	}
	assert.NotNil(t, c.C())

	var d *circD
	assert.NotPanics(t, func() {
		var ok bool
		i, err := injector.GetInstance(new(circD))
		assert.NoError(t, err)
		d, ok = i.(*circD)
		if !ok {
			t.Fail()
		}
	})

	assert.Panics(t, func() {
		d.A()
	})
}

func TestTracingDefaultsSeedNewInjectors(t *testing.T) { //nolint:paralleltest // mutates the package-level default
	previous := defaultCircularTracing.Load()

	EnableCircularTracing()

	defer func() {
		defaultCircularTracing.Store(previous)
	}()

	injector, err := NewInjector()
	assert.NoError(t, err)
	assert.True(t, injector.circularTracing)
}

func TestChildInheritsTracingState(t *testing.T) {
	t.Parallel()

	injector, err := NewInjector()
	assert.NoError(t, err)
	injector.EnableCircularTracing()
	injector.EnableInjectionTracing()

	child, err := injector.Child()
	assert.NoError(t, err)
	assert.True(t, child.circularTracing)
	assert.True(t, child.injectionTracing)
}

func TestTracingDefaultsConcurrentConstruction(t *testing.T) { //nolint:paralleltest // changes defaults and restores them after both workers finish
	circular, injection := defaultCircularTracing.Load(), defaultInjectionTracing.Load()

	t.Cleanup(func() {
		defaultCircularTracing.Store(circular)
		defaultInjectionTracing.Store(injection)
	})

	var workers sync.WaitGroup

	start := make(chan struct{})

	workers.Add(2)
	go func() {
		defer workers.Done()

		<-start

		for range 1000 {
			EnableCircularTracing()
			EnableInjectionTracing()
		}
	}()
	go func() {
		defer workers.Done()

		<-start

		for range 1000 {
			if _, err := NewInjector(); err != nil {
				t.Error(err)
				return
			}
		}
	}()

	close(start)
	workers.Wait()
}
