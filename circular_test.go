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
	traceCircular = make([]circularTraceEntry, 0)
	defer func() {
		traceCircular = nil
	}()

	injector := NewInjector()
	assert.Panics(t, func() {
		_, ok := injector.GetInstance(new(circA)).(*circA)
		if !ok {
			t.Fail()
		}
	})

	injector.Bind(new(circCInterface)).To(circC{})
	assert.NotPanics(t, func() {
		c, ok := injector.GetInstance(new(circC)).(*circC)
		if !ok {
			t.Fail()
		}

		assert.NotNil(t, c.C())
	})

	var d *circD
	assert.NotPanics(t, func() {
		var ok bool
		d, ok = injector.GetInstance(new(circD)).(*circD)
		if !ok {
			t.Fail()
		}
	})

	assert.Panics(t, func() {
		d.A()
	})
}
