//go:build !race

package contract_test

import (
	"fmt"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/require"
)

// The race detector adds allocations of its own, so these tests are not built
// with -race. Run them without it.

type (
	// perfService is the unscoped type: one injected field.
	perfService struct {
		Greeter greeter `inject:""`
	}

	perfSingleton struct {
		Greeter greeter `inject:""`
	}

	perfChildSingleton struct {
		Greeter greeter `inject:""`
	}

	perfServiceProvider func() *perfService

	perfProviderHolder struct {
		Provide perfServiceProvider `inject:""`
	}

	// perfConfig reads twenty configuration values.
	perfConfig struct {
		F01 string `inject:"config:perf.f01"`
		F02 string `inject:"config:perf.f02"`
		F03 string `inject:"config:perf.f03"`
		F04 string `inject:"config:perf.f04"`
		F05 string `inject:"config:perf.f05"`
		F06 string `inject:"config:perf.f06"`
		F07 string `inject:"config:perf.f07"`
		F08 string `inject:"config:perf.f08"`
		F09 string `inject:"config:perf.f09"`
		F10 string `inject:"config:perf.f10"`
		F11 int    `inject:"config:perf.f11"`
		F12 int    `inject:"config:perf.f12"`
		F13 int    `inject:"config:perf.f13"`
		F14 int    `inject:"config:perf.f14"`
		F15 int    `inject:"config:perf.f15"`
		F16 bool   `inject:"config:perf.f16"`
		F17 bool   `inject:"config:perf.f17"`
		F18 bool   `inject:"config:perf.f18"`
		F19 bool   `inject:"config:perf.f19"`
		F20 bool   `inject:"config:perf.f20"`
	}
)

// perfInjector binds what the allocation tests resolve.
func perfInjector(t *testing.T) *isolatedInjector {
	t.Helper()

	return newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(greeter)).To(new(helloGreeter))
		injector.Bind(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})
		injector.Bind(new(perfSingleton)).In(dingo.Singleton)
		injector.Bind(new(perfChildSingleton)).In(dingo.ChildSingleton)

		for i := 1; i <= 20; i++ {
			key := fmt.Sprintf("config:perf.f%02d", i)

			switch {
			case i <= 10:
				injector.Bind(new(string)).AnnotatedWith(key).ToInstance("value")
			case i <= 15:
				injector.Bind(new(int)).AnnotatedWith(key).ToInstance(i)
			default:
				injector.Bind(new(bool)).AnnotatedWith(key).ToInstance(true)
			}
		}
	}))
}

// requireAllocsAtMost fails the test when f allocates more than ceiling times
// per call. f runs once first, so one-time setup is not counted.
func requireAllocsAtMost(t *testing.T, ceiling float64, f func()) {
	t.Helper()

	f()

	allocs := testing.AllocsPerRun(100, f)

	t.Logf("allocations per call: %v, limit %v", allocs, ceiling)

	if allocs > ceiling {
		t.Fatalf("allocations per call grew: %v, limit %v", allocs, ceiling)
	}
}

func TestPerf_AllocsGetInstanceUnscoped(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts allocations of the whole process
	injector := perfInjector(t)

	requireAllocsAtMost(t, 10, func() {
		_, err := injector.GetInstance(new(perfService))
		require.NoError(t, err)
	})
}

func TestPerf_AllocsGetInstanceSingleton(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts allocations of the whole process
	injector := perfInjector(t)

	requireAllocsAtMost(t, 3, func() {
		_, err := injector.GetInstance(new(perfSingleton))
		require.NoError(t, err)
	})
}

func TestPerf_AllocsGetInstanceChildSingleton(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts allocations of the whole process
	injector := perfInjector(t).child(t)

	requireAllocsAtMost(t, 3, func() {
		_, err := injector.GetInstance(new(perfChildSingleton))
		require.NoError(t, err)
	})
}

func TestPerf_AllocsGetAnnotatedInstance(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts allocations of the whole process
	injector := perfInjector(t)

	requireAllocsAtMost(t, 5, func() {
		_, err := injector.GetAnnotatedInstance(new(greeter), "loud")
		require.NoError(t, err)
	})
}

func TestPerf_AllocsProviderCall(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts allocations of the whole process
	injector := perfInjector(t)

	instance, err := injector.GetInstance(new(perfProviderHolder))
	require.NoError(t, err)

	holder, ok := instance.(*perfProviderHolder)
	require.True(t, ok)

	requireAllocsAtMost(t, 11, func() {
		holder.Provide()
	})
}

func TestPerf_AllocsInjectTwentyConfigFields(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts allocations of the whole process
	injector := perfInjector(t)

	requireAllocsAtMost(t, 47, func() {
		require.NoError(t, injector.RequestInjection(&perfConfig{}))
	})
}
