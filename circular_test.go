package dingo

import (
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
	EnableCircularTracing()
	defer func() {
		traceCircular = nil
	}()

	injector, err := NewInjector()
	assert.NoError(t, err)

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

type (
	circSingletonA struct {
		B *circSingletonB `inject:""`
	}
	circSingletonB struct {
		A *circSingletonA `inject:""`
	}
)

func TestCircularSingletonBinding(t *testing.T) {
	EnableCircularTracing()
	defer func() {
		traceCircular = nil
	}()

	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.Bind(new(circSingletonA)).In(Singleton)
	injector.Bind(new(circSingletonB)).In(Singleton)

	assert.Panics(t, func() {
		injector.GetInstance(new(circSingletonA))
	}, "should panic on circular singleton dependency")
}


func TestConcurrentSingletonResolution(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.Bind(new(testSingleton)).In(Singleton)

	// Resolve the same singleton concurrently from multiple goroutines
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() {
			_, err := injector.GetInstance(new(testSingleton))
			errCh <- err
		}()
	}

	for i := 0; i < 20; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}
}
