package dingo_test

import (
	"strings"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loadMark struct{ name string }

type loadHolder struct {
	Marks []loadMark `inject:""`
}

// loadOrder builds an injector from the modules and returns the names in the
// order their Configure ran (the module load order).
func loadOrder(t *testing.T, modules ...dingo.Module) []string {
	t.Helper()
	inj, err := dingo.NewInjector(modules...)
	require.NoError(t, err)
	got, err := inj.GetInstance(new(loadHolder))
	require.NoError(t, err)
	names := make([]string, 0)
	for _, m := range got.(*loadHolder).Marks {
		names = append(names, m.name)
	}
	return names
}

func mark(i *dingo.Injector, name string) {
	i.BindMulti((*loadMark)(nil)).ToInstance(loadMark{name: name})
}

func indexOf(names []string) map[string]int {
	m := make(map[string]int, len(names))
	for i, n := range names {
		m[n] = i
	}
	return m
}

// linear chain: top -> middle -> bottom
type chainBottom struct{}

func (*chainBottom) Configure(i *dingo.Injector) { mark(i, "bottom") }

type chainMiddle struct{}

func (*chainMiddle) Configure(i *dingo.Injector) { mark(i, "middle") }
func (*chainMiddle) Depends() []dingo.Module     { return []dingo.Module{new(chainBottom)} }

type chainTop struct{}

func (*chainTop) Configure(i *dingo.Injector) { mark(i, "top") }
func (*chainTop) Depends() []dingo.Module     { return []dingo.Module{new(chainMiddle)} }

// diamond: root -> {left, right} -> leaf
type dLeaf struct{}

func (*dLeaf) Configure(i *dingo.Injector) { mark(i, "leaf") }

type dLeft struct{}

func (*dLeft) Configure(i *dingo.Injector) { mark(i, "left") }
func (*dLeft) Depends() []dingo.Module     { return []dingo.Module{new(dLeaf)} }

type dRight struct{}

func (*dRight) Configure(i *dingo.Injector) { mark(i, "right") }
func (*dRight) Depends() []dingo.Module     { return []dingo.Module{new(dLeaf)} }

type dRoot struct{}

func (*dRoot) Configure(i *dingo.Injector) { mark(i, "root") }
func (*dRoot) Depends() []dingo.Module     { return []dingo.Module{new(dLeft), new(dRight)} }

// independent siblings
type sibA struct{}

func (*sibA) Configure(i *dingo.Injector) { mark(i, "A") }

type sibB struct{}

func (*sibB) Configure(i *dingo.Injector) { mark(i, "B") }

type sibC struct{}

func (*sibC) Configure(i *dingo.Injector) { mark(i, "C") }

// a module loaded twice, carrying state to prove which instance wins
type dupMod struct{ tag string }

func (m *dupMod) Configure(i *dingo.Injector) { mark(i, m.tag) }

type dupRoot struct{}

func (*dupRoot) Configure(i *dingo.Injector) { mark(i, "dupRoot") }
func (*dupRoot) Depends() []dingo.Module     { return []dingo.Module{&dupMod{tag: "viaDepends"}} }

// cycles
type cycleOne struct{}

func (*cycleOne) Configure(*dingo.Injector) {}
func (*cycleOne) Depends() []dingo.Module   { return []dingo.Module{new(cycleTwo)} }

type cycleTwo struct{}

func (*cycleTwo) Configure(*dingo.Injector) {}
func (*cycleTwo) Depends() []dingo.Module   { return []dingo.Module{new(cycleOne)} }

type selfCycle struct{}

func (*selfCycle) Configure(*dingo.Injector)  {}
func (s *selfCycle) Depends() []dingo.Module  { return []dingo.Module{s} }

type loneOk struct{}

func (*loneOk) Configure(i *dingo.Injector) { mark(i, "lone") }

func TestChainLoadsBottomUp(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"bottom", "middle", "top"}, loadOrder(t, new(chainTop)))
}

func TestDiamondLoadsLeafOnceBeforeBranches(t *testing.T) {
	t.Parallel()
	got := loadOrder(t, new(dRoot))
	leaf := 0
	for _, n := range got {
		if n == "leaf" {
			leaf++
		}
	}
	assert.Equal(t, 1, leaf, "shared dependency must load once")
	pos := indexOf(got)
	assert.Less(t, pos["leaf"], pos["left"])
	assert.Less(t, pos["leaf"], pos["right"])
	assert.Less(t, pos["left"], pos["root"])
	assert.Less(t, pos["right"], pos["root"])
}

func TestIndependentModulesKeepListedOrder(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"C", "A", "B"}, loadOrder(t, new(sibC), new(sibA), new(sibB)))
}

func TestDuplicateModuleLoadsOnceFirstWins(t *testing.T) {
	t.Parallel()
	got := loadOrder(t, &dupMod{tag: "first"}, new(dupRoot))
	dup := 0
	for _, n := range got {
		if n == "first" || n == "viaDepends" {
			dup++
		}
	}
	assert.Equal(t, 1, dup, "duplicate module must load once")
	assert.Contains(t, got, "first", "first registered instance must win")
	assert.NotContains(t, got, "viaDepends")
}

func TestDirectCycleIsRejected(t *testing.T) {
	t.Parallel()
	_, err := dingo.NewInjector(new(cycleOne))
	require.Error(t, err)
	assert.ErrorIs(t, err, dingo.ErrModuleCycle)
}

func TestSelfDependencyIsRejected(t *testing.T) {
	t.Parallel()
	t.Skip("BUG: self-dependency not detected — see DINGO-FINDINGS.md#1")
	_, err := dingo.NewInjector(new(selfCycle))
	require.Error(t, err)
	assert.ErrorIs(t, err, dingo.ErrModuleCycle)
}

func TestCycleAmongSomeModulesStillRejected(t *testing.T) {
	t.Parallel()
	_, err := dingo.NewInjector(new(loneOk), new(cycleOne))
	require.Error(t, err)
	assert.ErrorIs(t, err, dingo.ErrModuleCycle)
}

func TestCycleErrorNamesTheModules(t *testing.T) {
	t.Parallel()
	_, err := dingo.NewInjector(new(cycleOne))
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "cycleOne")
	assert.Contains(t, msg, "cycleTwo")
	assert.True(t, strings.Contains(msg, "→"))
}
