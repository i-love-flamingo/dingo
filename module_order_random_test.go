package dingo_test

// This file is the ONLY randomized test in the suite. Everything else is a plain
// example test. If you are reading the suite to learn how dingo works, you can
// skip this file entirely — deleting it changes no other test.
//
// It stresses the two things dingo guarantees for EVERY module graph:
//   1. a module's dependencies (its Depends()) are configured BEFORE it;
//   2. the order is deterministic — the same wiring produces the same order
//      every build.
//
// dingo does NOT guarantee that independent modules keep the order you listed
// them: it sorts with gonum's DFS-based topo.SortStabilized, so a module can be
// pulled earlier when it is a dependency of an earlier-listed module — even a
// module with no dependencies of its own. The one case where listed order IS
// preserved (a set of modules with no dependency edges at all) is covered by a
// plain example test, TestIndependentModulesKeepListedOrder in
// module_loading_test.go. This file asserts only the two graph-independent
// guarantees above.
//
// It builds many small, randomly-wired module setups and checks both. On
// failure it prints the seed; re-run with DINGO_TEST_SEED=<that number> to
// reproduce.

import (
	"math/rand"
	"os"
	"strconv"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type orderMarker struct{ name string }

type orderHolder struct {
	Marks []orderMarker `inject:""`
}

// orderMod is the shared body of the little modules below: it records its name
// when configured and reports the dependencies it was given.
type orderMod struct {
	name string
	deps []dingo.Module
}

func (m orderMod) Configure(i *dingo.Injector) {
	i.BindMulti((*orderMarker)(nil)).ToInstance(orderMarker{name: m.name})
}
func (m orderMod) Depends() []dingo.Module   { return m.deps }
func (m *orderMod) setDeps(d []dingo.Module) { m.deps = d }

// Eight distinct module TYPES (dingo identifies modules by Go type).
type ordM0 struct{ orderMod }
type ordM1 struct{ orderMod }
type ordM2 struct{ orderMod }
type ordM3 struct{ orderMod }
type ordM4 struct{ orderMod }
type ordM5 struct{ orderMod }
type ordM6 struct{ orderMod }
type ordM7 struct{ orderMod }

const orderModCount = 8

func newOrderMod(i int, name string) dingo.Module {
	switch i {
	case 0:
		return &ordM0{orderMod{name: name}}
	case 1:
		return &ordM1{orderMod{name: name}}
	case 2:
		return &ordM2{orderMod{name: name}}
	case 3:
		return &ordM3{orderMod{name: name}}
	case 4:
		return &ordM4{orderMod{name: name}}
	case 5:
		return &ordM5{orderMod{name: name}}
	case 6:
		return &ordM6{orderMod{name: name}}
	case 7:
		return &ordM7{orderMod{name: name}}
	}
	panic("module index out of range")
}

func TestRandomModuleOrdering(t *testing.T) {
	seed := randomSeed(t)
	rng := rand.New(rand.NewSource(seed))

	for iter := 0; iter < 300; iter++ {
		mods := make([]dingo.Module, orderModCount)
		names := make([]string, orderModCount)
		for i := 0; i < orderModCount; i++ {
			names[i] = "m" + strconv.Itoa(i)
			mods[i] = newOrderMod(i, names[i])
		}

		// random dependencies: module j may depend on module i only when i < j
		// (never forms a cycle). deps[j] = indices j depends on.
		deps := make([][]int, orderModCount)
		for j := 0; j < orderModCount; j++ {
			for i := 0; i < j; i++ {
				if rng.Intn(3) == 0 {
					deps[j] = append(deps[j], i)
				}
			}
			depMods := make([]dingo.Module, 0, len(deps[j]))
			for _, i := range deps[j] {
				depMods = append(depMods, mods[i])
			}
			mods[j].(interface{ setDeps([]dingo.Module) }).setDeps(depMods)
		}

		listed := rng.Perm(orderModCount)
		args := make([]dingo.Module, orderModCount)
		for p, idx := range listed {
			args[p] = mods[idx]
		}

		// Build the injector several times with the same wiring; the load order
		// must be identical every time (deterministic — not dependent on Go map
		// iteration order), and every dependency must precede its dependent.
		var firstOrder []string
		for build := 0; build < 3; build++ {
			injector, err := dingo.NewInjector(args...)
			require.NoErrorf(t, err, "seed=%d iter=%d build=%d", seed, iter, build)
			holder, err := injector.GetInstance(new(orderHolder))
			require.NoErrorf(t, err, "seed=%d iter=%d build=%d", seed, iter, build)

			order := make([]string, 0, orderModCount)
			for _, mk := range holder.(*orderHolder).Marks {
				order = append(order, mk.name)
			}

			// determinism: every build of the same wiring yields the same order.
			if build > 0 {
				assert.Equalf(t, firstOrder, order,
					"seed=%d iter=%d: load order is not deterministic across builds", seed, iter)
				continue
			}
			firstOrder = order

			pos := map[string]int{}
			for p, name := range order {
				pos[name] = p
			}

			// dependencies configured before dependents.
			for j := 0; j < orderModCount; j++ {
				for _, i := range deps[j] {
					assert.Lessf(t, pos[names[i]], pos[names[j]],
						"seed=%d iter=%d: %s must load before %s", seed, iter, names[i], names[j])
				}
			}
		}
	}
}

func randomSeed(t *testing.T) int64 {
	t.Helper()
	if s := os.Getenv("DINGO_TEST_SEED"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		require.NoError(t, err, "DINGO_TEST_SEED must be an integer")
		t.Logf("using DINGO_TEST_SEED=%d", v)
		return v
	}
	const def = 1
	t.Logf("using default seed=%d (set DINGO_TEST_SEED to reproduce a failure)", def)
	return def
}
