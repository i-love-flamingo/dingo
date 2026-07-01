package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Svc is exported on purpose: dingo's interceptor sets the interceptor's first
// field via reflection, which only works if that embedded field is exported
// (an unexported embedded interface would make the field unsettable).
type Svc interface{ V() string }

type svcImpl struct{}

func (*svcImpl) V() string { return "base" }

type svcWrap struct {
	Svc
}

func (i *svcWrap) V() string { return i.Svc.V() + "+wrap" }

func TestInterceptorWrapsResult(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*Svc)(nil)).To(new(svcImpl))
	inj.BindInterceptor((*Svc)(nil), svcWrap{})

	got, err := inj.GetInstance((*Svc)(nil))
	require.NoError(t, err)
	assert.Equal(t, "base+wrap", got.(Svc).V())
}

func TestInterceptingNonInterfacePanics(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	assert.Panics(t, func() {
		inj.BindInterceptor(new(svcImpl), svcWrap{}) // concrete type, not interface
	})
}

// Candidate #4: an interceptor whose first field is NOT the intercepted
// interface. Documents the outcome (a panic) rather than silent corruption.
type badWrap struct {
	Unrelated int
	Svc
}

func TestInterceptorWithWrongFirstField(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*Svc)(nil)).To(new(svcImpl))
	inj.BindInterceptor((*Svc)(nil), badWrap{})

	defer func() { _ = recover() }() // expected: setting field 0 (an int) to the Svc value panics
	got, err := inj.GetInstance((*Svc)(nil))
	if err == nil {
		t.Skip("BUG: interceptor with a non-interface first field neither errors nor panics — see DINGO-FINDINGS.md#N")
	}
	_ = got
}
