package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type delayedTarget struct {
	Msg string `inject:""`
}

// requestingModule calls RequestInjection during Configure (init stage); the
// target must be injected after the injector finishes initializing.
type requestingModule struct {
	target *delayedTarget
}

func (m *requestingModule) Configure(i *dingo.Injector) {
	i.Bind((*string)(nil)).ToInstance("injected")
	_ = i.RequestInjection(m.target)
}

func TestRequestInjectionDelayedDuringInit(t *testing.T) {
	t.Parallel()
	target := &delayedTarget{}
	_, err := dingo.NewInjector(&requestingModule{target: target})
	require.NoError(t, err)
	assert.Equal(t, "injected", target.Msg, "object handed to RequestInjection during init must be injected")
}

func TestRequestInjectionImmediateAfterInit(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*string)(nil)).ToInstance("now")
	target := &delayedTarget{}
	require.NoError(t, inj.RequestInjection(target))
	assert.Equal(t, "now", target.Msg)
}

// A field that is itself a (non-pointer) struct with an inject tag is rejected.
type innerStruct struct {
	X string `inject:""`
}
type outerStruct struct {
	In innerStruct `inject:""`
}

func TestStructFieldInjectionRejected(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*string)(nil)).ToInstance("x")
	_, err = inj.GetInstance(new(outerStruct))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can not inject into struct")
}
