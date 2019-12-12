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
