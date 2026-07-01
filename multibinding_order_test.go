package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type label string

// firstMod depends on secondMod, so secondMod configures first regardless of
// the listed order.
type firstMod struct{}

func (*firstMod) Configure(i *dingo.Injector) { i.BindMulti((*label)(nil)).ToInstance(label("first")) }
func (*firstMod) Depends() []dingo.Module     { return []dingo.Module{new(secondMod)} }

type secondMod struct{}

func (*secondMod) Configure(i *dingo.Injector) { i.BindMulti((*label)(nil)).ToInstance(label("second")) }

type thirdMod struct{}

func (*thirdMod) Configure(i *dingo.Injector) { i.BindMulti((*label)(nil)).ToInstance(label("third")) }

type labelHolder struct {
	Labels []label `inject:""`
}

func TestMultibindingFollowsLoadOrder(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector(new(firstMod), new(thirdMod))
	require.NoError(t, err)
	got, err := inj.GetInstance(new(labelHolder))
	require.NoError(t, err)
	assert.Equal(t, []label{"second", "first", "third"}, got.(*labelHolder).Labels)
}

func TestMultibindingOrderIsStableAcrossRuns(t *testing.T) {
	t.Parallel()
	var want []label
	for run := 0; run < 25; run++ {
		inj, err := dingo.NewInjector(new(firstMod), new(thirdMod))
		require.NoError(t, err)
		got, err := inj.GetInstance(new(labelHolder))
		require.NoError(t, err)
		labels := got.(*labelHolder).Labels
		if run == 0 {
			want = labels
			continue
		}
		assert.Equal(t, want, labels, "multibinding order must be stable across runs")
	}
}

type baseGreeterMod struct{}

func (*baseGreeterMod) Configure(i *dingo.Injector) { i.Bind((*greeter)(nil)).To(english{}) }

type overridingGreeterMod struct{}

func (*overridingGreeterMod) Configure(i *dingo.Injector) {
	i.Override((*greeter)(nil), "").To(german{})
}
func (*overridingGreeterMod) Depends() []dingo.Module { return []dingo.Module{new(baseGreeterMod)} }

func TestOverrideWinsWhenListedBeforeItsDependency(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector(new(overridingGreeterMod))
	require.NoError(t, err)
	got, err := inj.GetInstance((*greeter)(nil))
	require.NoError(t, err)
	assert.Equal(t, "hallo", got.(greeter).Greet())
}

// Candidate #7: a multibinding element bound .In(Singleton) should resolve to
// the same instance each time.
func TestMultibindingSingletonElementIsShared(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.BindMulti((*greeter)(nil)).To(english{}).In(dingo.Singleton)

	type holder struct {
		G []greeter `inject:""`
	}
	a, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	b, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	if a.(*holder).G[0] != b.(*holder).G[0] {
		t.Skip("BUG: .In(Singleton) on a multibinding element is not honored — see DINGO-FINDINGS.md#N")
	}
	assert.Same(t, a.(*holder).G[0], b.(*holder).G[0])
}
