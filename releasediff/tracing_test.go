package releasediff_test

import (
	"fmt"
	"sync"
	"sync/atomic"

	"flamingo.me/dingo"
)

// Probes on circular dependency tracing and injection logging. Without
// tracing a cycle overflows the stack; with tracing dingo reports it. Flamingo
// switches tracing on from config, so it may come after injectors exist.

func init() {
	register("trace_enable_after_injector_created", probeTraceEnableAfterInjectorCreated)
	register("trace_enable_after_child_created", probeTraceEnableAfterChildCreated)
	register("trace_enable_then_new_child", probeTraceEnableThenNewChild)
	register("trace_struct_cycle", probeTraceStructCycle)
	register("trace_cycle_through_provider_binding", probeTraceCycleThroughProviderBinding)
	register("trace_singleton_cycle", probeTraceSingletonCycle)
	register("trace_provider_recursion_depth_10", func(r *recorder) { probeTraceProviderRecursion(r, 10) })
	register("trace_provider_recursion_depth_20", func(r *recorder) { probeTraceProviderRecursion(r, 20) })
	register("trace_injection_logging_enabled_after_create", probeTraceInjectionLoggingAfterCreate)
	register("trace_injection_logging_enabled_before_create", probeTraceInjectionLoggingBeforeCreate)
	register("trace_legit_graph", probeTraceLegitGraph)
}

type (
	CycleA struct {
		B *CycleB `inject:""`
	}
	CycleB struct {
		A *CycleA `inject:""`
	}
)

// probeTraceEnableAfterInjectorCreated enables tracing after the injector
// exists, then resolves a cycle.
func probeTraceEnableAfterInjectorCreated(r *recorder) {
	injector, _ := dingo.NewInjector()

	dingo.EnableCircularTracing()
	r.add("get", catch(func() string { return result(injector.GetInstance(new(CycleA))) }))
}

// probeTraceEnableAfterChildCreated enables tracing after a child exists,
// then resolves a cycle in the child.
func probeTraceEnableAfterChildCreated(r *recorder) {
	root, _ := dingo.NewInjector()
	child, _ := root.Child()

	dingo.EnableCircularTracing()
	r.add("get", catch(func() string { return result(child.GetInstance(new(CycleA))) }))
}

// probeTraceEnableThenNewChild enables tracing after the root exists and
// before its child is created.
func probeTraceEnableThenNewChild(r *recorder) {
	root, _ := dingo.NewInjector()

	dingo.EnableCircularTracing()

	child, _ := root.Child()

	r.add("get", catch(func() string { return result(child.GetInstance(new(CycleA))) }))
}

// probeTraceStructCycle resolves two structs that inject each other.
func probeTraceStructCycle(r *recorder) {
	injector, _ := dingo.NewInjector()

	r.add("get", catch(func() string { return result(injector.GetInstance(new(CycleA))) }))
}

type (
	PortI    interface{ Port() }
	portImpl struct{}
	PortUser struct {
		P PortI `inject:""`
	}
)

func (portImpl) Port() {}

// probeTraceCycleThroughProviderBinding closes a cycle through a ToProvider
// argument.
func probeTraceCycleThroughProviderBinding(r *recorder) {
	injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(PortI)).ToProvider(func(*PortUser) PortI { return portImpl{} })
	}))

	r.add("get", catch(func() string { return result(injector.GetInstance(new(PortUser))) }))
}

type (
	SingletonA struct {
		B *SingletonB `inject:""`
	}
	SingletonB struct {
		A *SingletonA `inject:""`
	}
)

// probeTraceSingletonCycle closes a cycle through a singleton, which can
// block on the scope's own lock.
func probeTraceSingletonCycle(r *recorder) {
	injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(SingletonA)).In(dingo.Singleton)
	}))

	r.add("get", catch(func() string { return result(injector.GetInstance(new(SingletonA))) }))
}

// RecursiveNode asks for another node through a generated provider until the
// limit is reached: recursion that ends, which tracing may still report.
type (
	RecursiveNodeProvider func() *RecursiveNode
	RecursiveNode         struct{ depth int64 }
)

var recursionDepth, recursionLimit atomic.Int64

func (n *RecursiveNode) Inject(p RecursiveNodeProvider) {
	n.depth = recursionDepth.Add(1)
	if n.depth < recursionLimit.Load() {
		p()
	}
}

func probeTraceProviderRecursion(r *recorder, limit int64) {
	recursionLimit.Store(limit)

	injector, _ := dingo.NewInjector()

	r.add("get", catch(func() string {
		v, err := injector.GetInstance(new(RecursiveNode))

		return errText(err) + fmt.Sprintf(" %T", v)
	}))
	r.add("depth", fmt.Sprint(recursionDepth.Load()))
}

// probeTraceInjectionLoggingAfterCreate enables injection logging after the
// injector exists and counts the log lines.
func probeTraceInjectionLoggingAfterCreate(r *recorder) {
	injector, _ := dingo.NewInjector()

	dingo.EnableInjectionTracing()

	_, err := injector.GetInstance(new(Mid))
	r.add("get", errText(err))
	r.add("INJECTING-lines", fmt.Sprint(countLogLines("INJECTING")))
	r.add("SETTING-lines", fmt.Sprint(countLogLines("SETTING FIELD")))
}

// probeTraceInjectionLoggingBeforeCreate enables injection logging first and
// resolves in a child.
func probeTraceInjectionLoggingBeforeCreate(r *recorder) {
	dingo.EnableInjectionTracing()

	injector, _ := dingo.NewInjector()
	child, _ := injector.Child()

	_, err := child.GetInstance(new(Mid))
	r.add("get", errText(err))
	r.add("INJECTING-lines", fmt.Sprint(countLogLines("INJECTING")))
	r.add("SETTING-lines", fmt.Sprint(countLogLines("SETTING FIELD")))
}

// A diamond with providers and scopes and no cycle; tracing must not report
// one.
type (
	LeafProvider func() *Leaf
	DiamondTop   struct {
		A    *DiamondA    `inject:""`
		B    *DiamondB    `inject:""`
		Leaf LeafProvider `inject:""`
	}
	DiamondA struct {
		S *Shared `inject:""`
	}
	DiamondB struct {
		S *Shared `inject:""`
		H Handler `inject:""`
	}
	Shared struct {
		L *Leaf `inject:""`
	}
)

// probeTraceLegitGraph resolves the diamond in two areas, and concurrently.
func probeTraceLegitGraph(r *recorder) {
	root := mustArea(nil, map[string]any{"site": "root"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Shared)).In(dingo.Singleton)
		injector.Bind(new(Handler)).To(siteHandler{}).In(dingo.ChildSingleton)
	}))
	fr := mustArea(root, map[string]any{"site": "fr"})

	for _, a := range []namedArea{{"root", root}, {"fr", fr}} {
		r.add(a.name, catch(func() string {
			v, err := a.injector.GetInstance(new(DiamondTop))
			if err != nil {
				return errText(err)
			}

			top := v.(*DiamondTop)

			return fmt.Sprintf("shared=%s handler=%s leaf=%T", identity(top.A.S, top.B.S), top.B.H.Name(), top.Leaf())
		}))
	}

	r.add("concurrent", catch(func() string {
		v, err := fr.GetInstance(new(DiamondTop))
		if err != nil {
			return errText(err)
		}

		top := v.(*DiamondTop)

		var (
			wg  sync.WaitGroup
			bad atomic.Int64
		)

		for range 32 {
			wg.Go(func() {
				if top.Leaf() == nil {
					bad.Add(1)
				}

				if s, _ := fr.GetInstance(new(Shared)); s != top.A.S {
					bad.Add(1)
				}
			})
		}

		wg.Wait()

		return fmt.Sprintf("bad=%d", bad.Load())
	}))
}
