package dingo_test

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"flamingo.me/dingo"
)

const traceRecursionLimit = 32

type (
	traceService           interface{ Origin() *traceOrigin }
	traceServiceProvider   func() traceService
	traceFallibleProvider  func() (traceService, error)
	traceSliceProvider     func() []traceService
	traceMapProvider       func() map[string]traceService
	traceProvidersProvider func() *traceProviders
	traceProviders         struct {
		Direct        traceServiceProvider             `inject:""`
		Fallible      traceFallibleProvider            `inject:""`
		Slice         []traceServiceProvider           `inject:""`
		Map           map[string]traceServiceProvider  `inject:""`
		FallibleSlice []traceFallibleProvider          `inject:""`
		FallibleMap   map[string]traceFallibleProvider `inject:""`
		SliceFactory  traceSliceProvider               `inject:""`
		MapFactory    traceMapProvider                 `inject:""`
		Again         traceProvidersProvider           `inject:""`
	}
	traceConfig struct {
		calls    atomic.Int32
		onOrigin func(*traceProviders)
	}
	traceOrigin  struct{ Providers *traceProviders }
	traceProduct struct {
		Root *traceOrigin `inject:""`
	}
)

func (product *traceProduct) Origin() *traceOrigin { return product.Root }

func (origin *traceOrigin) Inject(config *traceConfig, providers *traceProviders) {
	if config.calls.Add(1) > traceRecursionLimit {
		panic("recursion guard")
	}

	origin.Providers = providers
	if config.onOrigin != nil {
		config.onOrigin(providers)
	}
}

func traceInjector(t *testing.T, config *traceConfig) *dingo.Injector {
	t.Helper()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	injector.Bind(new(traceConfig)).ToInstance(config)
	injector.Bind(new(traceService)).To(traceProduct{})
	injector.BindMulti(new(traceService)).To(traceProduct{})
	injector.BindMap(new(traceService), "product").To(traceProduct{})

	return injector
}

func tracePanic(call func()) (result any) {
	defer func() { result = recover() }()

	call()
	return nil
}

func traceCalls() map[string]func(*traceProviders) traceService {
	return map[string]func(*traceProviders) traceService{
		"direct": func(providers *traceProviders) traceService { return providers.Direct() },
		"fallible": func(providers *traceProviders) traceService {
			product, err := providers.Fallible()
			if err != nil {
				panic(err)
			}

			return product
		},
		"slice": func(providers *traceProviders) traceService { return providers.Slice[0]() },
		"map":   func(providers *traceProviders) traceService { return providers.Map["product"]() },
		"fallible slice": func(providers *traceProviders) traceService {
			product, err := providers.FallibleSlice[0]()
			if err != nil {
				panic(err)
			}

			return product
		},
		"fallible map": func(providers *traceProviders) traceService {
			product, err := providers.FallibleMap["product"]()
			if err != nil {
				panic(err)
			}

			return product
		},
		"slice factory": func(providers *traceProviders) traceService { return providers.SliceFactory()[0] },
		"map factory":   func(providers *traceProviders) traceService { return providers.MapFactory()["product"] },
	}
}

func TestTracingDetectsSynchronousProviderCycles(t *testing.T) {
	t.Parallel()

	for name, call := range traceCalls() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := &traceConfig{onOrigin: func(providers *traceProviders) { call(providers) }}
			parent := traceInjector(t, config)
			child, err := parent.Child()
			require.NoError(t, err)
			child.EnableCircularTracing()

			got := tracePanic(func() { _, _ = child.GetInstance(new(traceOrigin)) })
			require.Equal(t, "detected circular dependency", got)
			require.EqualValues(t, dingo.ProviderReentryLimit+1, config.calls.Load())
		})
	}
}

func TestTracingAllowsDeferredProviders(t *testing.T) {
	t.Parallel()

	for name, call := range traceCalls() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			injector := traceInjector(t, &traceConfig{})
			injector.EnableCircularTracing()
			instance, err := injector.GetInstance(new(traceOrigin))
			require.NoError(t, err)

			origin := instance.(*traceOrigin)

			results := make(chan traceService, 2)
			for range cap(results) {
				go func() { results <- call(origin.Providers) }()
			}

			for range cap(results) {
				product := traceReceive(t, results)
				require.NotNil(t, product.Origin())
				require.NotSame(t, origin, product.Origin())
			}
		})
	}
}

func TestTracingDropsFinishedInnerConstruction(t *testing.T) {
	t.Parallel()

	config := &traceConfig{onOrigin: func(providers *traceProviders) {
		require.NotNil(t, providers.Again())
	}}
	injector := traceInjector(t, config)
	injector.EnableCircularTracing()
	_, err := injector.GetInstance(new(traceOrigin))
	require.NoError(t, err)
}

func TestTracingReleasesConstructionAfterPanic(t *testing.T) {
	t.Parallel()

	var saved *traceProviders

	config := &traceConfig{onOrigin: func(providers *traceProviders) {
		saved = providers

		panic("construction failed")
	}}
	injector := traceInjector(t, config)
	injector.EnableCircularTracing()
	require.Equal(t, "construction failed", tracePanic(func() { _, _ = injector.GetInstance(new(traceOrigin)) }))

	config.onOrigin = nil

	require.NotNil(t, saved.Direct().Origin())
}

type (
	traceMissing       interface{ Present() }
	tracePresent       struct{}
	traceErrorProvider func() *traceErrorOrigin
	traceErrorConfig   struct{ saved traceErrorProvider }
	traceErrorOrigin   struct {
		Missing traceMissing `inject:""`
	}
)

func (tracePresent) Present() {}
func (*traceErrorOrigin) Inject(config *traceErrorConfig, provider traceErrorProvider) {
	config.saved = provider
}

func TestTracingReleasesConstructionAfterError(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	injector.EnableCircularTracing()

	config := &traceErrorConfig{}
	injector.Bind(new(traceErrorConfig)).ToInstance(config)
	_, err = injector.GetInstance(new(traceErrorOrigin))
	require.ErrorContains(t, err, "can not instantiate interface")
	injector.Bind(new(traceMissing)).ToInstance(tracePresent{})
	require.NotNil(t, config.saved().Missing)
}

func TestTracingCanBeEnabledAfterProviderSynthesis(t *testing.T) {
	t.Parallel()

	for name, call := range traceCalls() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := &traceConfig{}
			injector := traceInjector(t, config)
			instance, err := injector.GetInstance(new(traceOrigin))
			require.NoError(t, err)

			config.onOrigin = func(providers *traceProviders) { call(providers) }

			injector.EnableCircularTracing()
			require.Equal(t, "detected circular dependency", tracePanic(func() {
				call(instance.(*traceOrigin).Providers)
			}))
		})
	}
}

func TestTracingDetectsUnboundedCycleAcrossGoroutines(t *testing.T) {
	t.Parallel()

	config := &traceConfig{onOrigin: func(providers *traceProviders) {
		results := make(chan any, 1)
		go func() { results <- tracePanic(func() { providers.Direct() }) }()

		if result := traceReceive(t, results); result != nil {
			panic(result)
		}
	}}
	injector := traceInjector(t, config)
	injector.EnableCircularTracing()
	require.Equal(t, "detected circular dependency", tracePanic(func() { _, _ = injector.GetInstance(new(traceOrigin)) }))
	require.EqualValues(t, dingo.ProviderReentryLimit+1, config.calls.Load())
}

func traceReceive[T any](t *testing.T, channel <-chan T) T {
	t.Helper()

	select {
	case value := <-channel:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for provider")

		var zero T
		return zero
	}
}

type (
	traceBranch         struct{}
	traceBranchProvider func() *traceBranch
	traceBranchConfig   struct {
		entered chan struct{}
		release chan struct{}
	}
	traceConcurrentOrigin struct{}
	traceConcurrentConfig struct{ run func(traceBranchProvider) }
)

func (*traceBranch) Inject(config *traceBranchConfig) {
	config.entered <- struct{}{}

	<-config.release
}
func (*traceConcurrentOrigin) Inject(config *traceConcurrentConfig, provider traceBranchProvider) {
	config.run(provider)
}

func TestTracingConcurrentProviderBranchesAreIndependent(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	injector.EnableCircularTracing()

	branch := &traceBranchConfig{entered: make(chan struct{}, 2), release: make(chan struct{})}

	t.Cleanup(func() { close(branch.release) })
	injector.Bind(new(traceBranchConfig)).ToInstance(branch)

	results := make(chan any, 2)

	injector.Bind(new(traceConcurrentConfig)).ToInstance(&traceConcurrentConfig{run: func(provider traceBranchProvider) {
		for range cap(results) {
			go func() { results <- tracePanic(func() { provider() }) }()
		}

		for range cap(results) {
			traceReceive(t, branch.entered)
		}

		for range cap(results) {
			branch.release <- struct{}{}
		}

		for range cap(results) {
			require.Nil(t, traceReceive(t, results))
		}
	}})
	_, err = injector.GetInstance(new(traceConcurrentOrigin))
	require.NoError(t, err)
}

func TestTracingIgnoresAncestorThatFinishesDuringProviderCall(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{}, 1)
	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	results := make(chan any, 1)
	config := &traceConfig{}
	config.onOrigin = func(providers *traceProviders) {
		if config.calls.Load() != 1 {
			return
		}

		go func() { results <- tracePanic(func() { providers.Direct() }) }()

		traceReceive(t, entered)
	}
	injector := traceInjector(t, config)
	injector.EnableCircularTracing()
	injector.Override(new(traceService), "").ToProvider(func() traceService {
		entered <- struct{}{}

		<-release
		return &traceProduct{}
	})
	require.NoError(t, injector.InitModules())
	_, err := injector.GetInstance(new(traceOrigin))
	require.NoError(t, err)

	release <- struct{}{}

	require.Nil(t, traceReceive(t, results))
}

type (
	TraceIntercepted      interface{ Value() int }
	traceInterceptedValue struct{}
	traceInterceptor      struct {
		TraceIntercepted
		Cycle *traceSelfCycle `inject:""`
	}
	traceProvidedInterceptor struct {
		TraceIntercepted
		Dependency traceInterceptDependency `inject:""`
	}
	traceInterceptDependency      interface{ Dependency() }
	traceInterceptDependencyValue struct {
		Cycle *traceSelfCycle `inject:""`
	}
	traceSelfCycle struct {
		Next *traceSelfCycle `inject:""`
	}
	traceCycleCount struct{ calls atomic.Int32 }
)

func (traceInterceptedValue) Value() int           { return 1 }
func (*traceInterceptDependencyValue) Dependency() {}
func (*traceSelfCycle) Inject(count *traceCycleCount) {
	if count.calls.Add(1) > traceRecursionLimit {
		panic("recursion guard")
	}
}

func TestTracingChildCarriesTraceThroughParentInterceptor(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"direct", "provider argument", "provider output"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			parent, err := dingo.NewInjector()
			require.NoError(t, err)

			count := &traceCycleCount{}
			parent.Bind(new(traceCycleCount)).ToInstance(count)
			parent.Bind(new(TraceIntercepted)).ToInstance(traceInterceptedValue{})

			switch path {
			case "direct":
				parent.BindInterceptor(new(TraceIntercepted), traceInterceptor{})
			case "provider argument":
				parent.BindInterceptor(new(TraceIntercepted), traceProvidedInterceptor{})
				parent.Bind(new(traceInterceptDependency)).ToProvider(func(cycle *traceSelfCycle) traceInterceptDependency {
					return &traceInterceptDependencyValue{Cycle: cycle}
				})
			case "provider output":
				parent.BindInterceptor(new(TraceIntercepted), traceProvidedInterceptor{})
				parent.Bind(new(traceInterceptDependency)).ToProvider(func() traceInterceptDependency {
					return &traceInterceptDependencyValue{}
				})
			}

			child, err := parent.Child()
			require.NoError(t, err)
			child.EnableCircularTracing()
			require.Equal(t, "detected circular dependency", tracePanic(func() { _, _ = child.GetInstance(new(TraceIntercepted)) }))
			count.calls.Store(0)
			require.Equal(t, "recursion guard", tracePanic(func() { _, _ = parent.GetInstance(new(traceSelfCycle)) }))
		})
	}
}

type (
	traceCycleProvider    func() *traceSelfCycle
	traceSavedInterceptor struct {
		TraceIntercepted
		Make traceCycleProvider `inject:""`
	}
	traceInterceptedOrigin struct {
		Service TraceIntercepted `inject:""`
	}
)

func TestTracingDeferredParentProviderRetainsEnabledTrace(t *testing.T) {
	t.Parallel()

	parent, err := dingo.NewInjector()
	require.NoError(t, err)
	parent.Bind(new(traceCycleCount)).ToInstance(&traceCycleCount{})
	parent.Bind(new(TraceIntercepted)).ToInstance(traceInterceptedValue{})
	parent.BindInterceptor(new(TraceIntercepted), traceSavedInterceptor{})
	child, err := parent.Child()
	require.NoError(t, err)
	child.EnableCircularTracing()
	instance, err := child.GetInstance(new(traceInterceptedOrigin))
	require.NoError(t, err)

	interceptor := instance.(*traceInterceptedOrigin).Service.(*traceSavedInterceptor)

	require.Equal(t, "detected circular dependency", tracePanic(func() { interceptor.Make() }))
}

type (
	overlapRoot struct {
		X    *overlapX    `inject:""`
		Slow *overlapSlow `inject:""`
	}
	overlapX struct {
		TP overlapTProvider `inject:""`
	}
	overlapTProvider func() *overlapT
	overlapT         struct {
		Root *overlapRoot `inject:""`
	}
	overlapSlow struct{}
	overlapGate struct {
		first   atomic.Bool
		entered chan struct{}
		release chan struct{}
	}
)

func (*overlapSlow) Inject(gate *overlapGate) {
	if gate.first.CompareAndSwap(false, true) {
		gate.entered <- struct{}{}

		<-gate.release
	}
}

func TestTracingOverlappingProviderCallIsNotACycle(t *testing.T) {
	t.Parallel()

	for name, scope := range map[string]dingo.Scope{
		"unscoped root":        nil,
		"singleton root":       dingo.NewSingletonScope(),
		"child singleton root": dingo.NewChildSingletonScope(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			injector, err := dingo.NewInjector()
			require.NoError(t, err)
			injector.BindScope(dingo.NewSingletonScope())
			injector.EnableCircularTracing()

			gate := &overlapGate{entered: make(chan struct{}, 1), release: make(chan struct{})}
			injector.Bind(new(overlapGate)).ToInstance(gate)
			injector.Bind(new(overlapX)).In(dingo.Singleton)

			if scope != nil {
				injector.BindScope(scope)
				injector.Bind(new(overlapRoot)).In(scope)
			}

			errs := make(chan error, 1)

			go func() {
				_, err := injector.GetInstance(new(overlapRoot))
				errs <- err
			}()

			traceReceive(t, gate.entered)

			instance, err := injector.GetInstance(new(overlapX))
			require.NoError(t, err)

			// A scoped root makes the call wait until the root is built.
			results := make(chan any, 1)

			go func() {
				results <- tracePanic(func() {
					if instance.(*overlapX).TP().Root == nil {
						panic("missing root")
					}
				})
			}()

			time.Sleep(20 * time.Millisecond)
			close(gate.release)
			require.Nil(t, traceReceive(t, results))
			require.NoError(t, traceReceive(t, errs))
		})
	}
}

type (
	crowdRoot struct {
		X       *crowdX       `inject:""`
		Slow    *overlapSlow  `inject:""`
		Barrier *crowdBarrier `inject:""`
	}
	crowdX struct {
		TP crowdTProvider `inject:""`
	}
	crowdTProvider func() *crowdT
	crowdT         struct {
		Root *crowdRoot `inject:""`
	}
	crowdBarrier struct{}
	crowdGate    struct {
		callers int32
		arrived atomic.Int32
		all     chan struct{}
	}
)

// Inject keeps every nested root open until all callers have one.
func (*crowdBarrier) Inject(gate *crowdGate, slow *overlapGate) {
	if !slow.first.Load() {
		return
	}

	if gate.arrived.Add(1) == gate.callers {
		close(gate.all)
	}

	select {
	case <-gate.all:
	case <-time.After(5 * time.Second):
	}
}

func TestTracingManyOverlappingProviderCallsAreNotACycle(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	injector.BindScope(dingo.NewSingletonScope())
	injector.EnableCircularTracing()

	callers := 2 * dingo.ProviderReentryLimit
	slow := &overlapGate{entered: make(chan struct{}, 1), release: make(chan struct{})}
	gate := &crowdGate{callers: int32(callers), all: make(chan struct{})}

	injector.Bind(new(overlapGate)).ToInstance(slow)
	injector.Bind(new(crowdGate)).ToInstance(gate)
	injector.Bind(new(crowdX)).In(dingo.Singleton)

	errs := make(chan error, 1)

	go func() {
		_, err := injector.GetInstance(new(crowdRoot))
		errs <- err
	}()

	traceReceive(t, slow.entered)

	instance, err := injector.GetInstance(new(crowdX))
	require.NoError(t, err)

	results := make(chan any, callers)
	for range callers {
		go func() { results <- tracePanic(func() { instance.(*crowdX).TP() }) }()
	}

	for range callers {
		require.Nil(t, traceReceive(t, results))
	}

	close(slow.release)
	require.NoError(t, traceReceive(t, errs))
}

type (
	twoScopesI    interface{}
	twoScopesJ    interface{}
	twoScopesK    interface{}
	twoScopesPeer interface{}
	twoScopesA    struct {
		Peer twoScopesPeer `inject:",optional"`
	}
	twoScopesLink struct {
		J twoScopesJ `inject:""`
	}
)

func TestTracingTellsScopesApart(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	singleton := dingo.NewSingletonScope()
	injector.BindScope(singleton)
	injector.EnableCircularTracing()
	injector.Bind(new(twoScopesI)).To(twoScopesA{}).In(dingo.ChildSingleton)
	injector.Bind(new(twoScopesJ)).To(twoScopesA{}).In(dingo.Singleton)
	injector.Bind(new(twoScopesK)).To(twoScopesA{}).In(dingo.Singleton)

	// The singleton A is built first, through K and without a peer.
	_, err = injector.GetInstance(new(twoScopesK))
	require.NoError(t, err)

	child, err := injector.Child()
	require.NoError(t, err)
	child.BindScope(singleton)
	child.Bind(new(twoScopesPeer)).To(twoScopesLink{})

	// The child singleton A reaches the cached singleton A through its peer and J.
	require.Nil(t, tracePanic(func() {
		_, err := child.GetInstance(new(twoScopesI))
		require.NoError(t, err)
	}))
}

// passScope is a custom scope that takes no lock.
type passScope struct{}

func (passScope) ResolveType(t reflect.Type, annotation string, unscoped func(reflect.Type, string, bool) (reflect.Value, error)) (reflect.Value, error) {
	return unscoped(t, annotation, false)
}

type (
	revX struct {
		Y *revY `inject:""`
	}
	revY         struct{}
	revXProvider func() *revX
)

func TestTracingAllowsBoundedProviderReentry(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		depth  int
		scope  dingo.Scope
		panics bool
	}{
		"depth 1":                      {depth: 1},
		"at limit":                     {depth: dingo.ProviderReentryLimit},
		"above limit":                  {depth: dingo.ProviderReentryLimit + 1, panics: true},
		"depth 1 in a lock-free scope": {depth: 1, scope: passScope{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			injector, err := dingo.NewInjector()
			require.NoError(t, err)
			injector.EnableCircularTracing()

			depth := 0

			binding := injector.Bind(new(revY)).ToProvider(func(xp revXProvider) *revY {
				if depth < test.depth {
					depth++

					xp()
				}

				return &revY{}
			})
			if test.scope != nil {
				injector.BindScope(test.scope)
				binding.In(test.scope)
			}

			got := tracePanic(func() {
				_, err := injector.GetInstance(new(revX))
				require.NoError(t, err)
			})
			if test.panics {
				require.Equal(t, "detected circular dependency", got)
			} else {
				require.Nil(t, got)
				require.Equal(t, test.depth, depth)
			}
		})
	}
}

type (
	scopedCycleCount struct{ calls atomic.Int32 }
	scopedCycleA     struct {
		B *scopedCycleB `inject:""`
	}
	scopedCycleB struct{}
)

func (count *scopedCycleCount) guard() {
	if count.calls.Add(1) > traceRecursionLimit {
		panic("recursion guard")
	}
}

func (*scopedCycleB) Inject(a *scopedCycleA, count *scopedCycleCount) {
	count.guard()
}

func TestTracingReportsDirectScopedCycle(t *testing.T) {
	t.Parallel()

	for name, scope := range map[string]dingo.Scope{
		"singleton":       dingo.NewSingletonScope(),
		"child singleton": dingo.NewChildSingletonScope(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			injector, err := dingo.NewInjector()
			require.NoError(t, err)
			injector.BindScope(scope)
			injector.EnableCircularTracing()

			count := &scopedCycleCount{}
			injector.Bind(new(scopedCycleCount)).ToInstance(count)
			injector.Bind(new(scopedCycleA)).In(scope)

			// A missed cycle blocks forever on the scope lock, so it runs apart.
			results := make(chan any, 1)

			go func() { results <- tracePanic(func() { _, _ = injector.GetInstance(new(scopedCycleA)) }) }()

			require.Equal(t, "detected circular dependency", traceReceive(t, results))
			require.EqualValues(t, 0, count.calls.Load())
		})
	}
}

type (
	heldRoot struct {
		Holder *heldHolder `inject:""`
		Caller *heldCaller `inject:""`
	}
	heldHolder struct {
		Make heldTailProvider `inject:""`
	}
	heldTailProvider func() *heldTail
	heldTail         struct {
		Root *heldRoot `inject:""`
	}
	heldCaller struct{}
)

func (*heldCaller) Inject(holder *heldHolder, count *scopedCycleCount) {
	count.guard()
	holder.Make()
}

// A provider keeps the ancestry it was created with. A cycle that keeps calling
// one long-lived provider looks like overlapping calls, so it is not reported.
func TestTracingMissesCycleThroughLongLivedProvider(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	injector.BindScope(dingo.NewSingletonScope())
	injector.EnableCircularTracing()

	count := &scopedCycleCount{}
	injector.Bind(new(scopedCycleCount)).ToInstance(count)
	injector.Bind(new(heldHolder)).In(dingo.Singleton)

	require.Equal(t, "recursion guard", tracePanic(func() { _, _ = injector.GetInstance(new(heldRoot)) }))
}

func TestTracingSequentialProviderReentriesDoNotAddUp(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	injector.EnableCircularTracing()

	calls := 0

	injector.Bind(new(revY)).ToProvider(func(xp revXProvider) *revY {
		if calls == 0 {
			for range 2 * dingo.ProviderReentryLimit {
				calls++

				xp()
			}
		}

		return &revY{}
	})

	_, err = injector.GetInstance(new(revX))
	require.NoError(t, err)
	require.Equal(t, 2*dingo.ProviderReentryLimit, calls)
}
