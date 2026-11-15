package releasediff_test

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"flamingo.me/dingo"
)

// Probes on modules: Depends ordering, deduplication, cycles, module field
// injection and TryModule.

func init() {
	register("module_self_dependency", probeModuleSelfDependency)
	register("module_cycle", probeModuleCycle)
	register("module_configure_order", probeModuleConfigureOrder)
	register("module_field_injection_from_area_config", probeModuleFieldInjectionFromAreaConfig)
	register("module_trymodule", probeModuleTryModule)
}

// recorded lists what modules did, in order.
var (
	recordedMu sync.Mutex
	recorded   []string
)

func record(s string) {
	recordedMu.Lock()
	defer recordedMu.Unlock()

	recorded = append(recorded, s)
}

// takeRecorded returns and clears what modules recorded.
func takeRecorded() string {
	recordedMu.Lock()
	defer recordedMu.Unlock()

	s := strings.Join(recorded, ",")
	recorded = nil

	return s
}

type outerModule struct{}

func (*outerModule) Configure(*dingo.Injector) { record("outer") }
func (*outerModule) Depends() []dingo.Module   { return []dingo.Module{new(selfModule)} }

// A diamond with an extra branch: top depends on left and right, both on
// base, right also on late.
type (
	topModule   struct{}
	leftModule  struct{}
	rightModule struct{}
	baseModule  struct{}
	otherModule struct{}
	lateModule  struct{}
)

func (*topModule) Configure(*dingo.Injector)   { record("top") }
func (*topModule) Depends() []dingo.Module     { return []dingo.Module{new(leftModule), new(rightModule)} }
func (*leftModule) Configure(*dingo.Injector)  { record("left") }
func (*leftModule) Depends() []dingo.Module    { return []dingo.Module{new(baseModule)} }
func (*rightModule) Configure(*dingo.Injector) { record("right") }
func (*rightModule) Depends() []dingo.Module   { return []dingo.Module{new(baseModule), new(lateModule)} }
func (*baseModule) Configure(*dingo.Injector)  { record("base") }
func (*otherModule) Configure(*dingo.Injector) { record("other") }
func (*lateModule) Configure(*dingo.Injector)  { record("late") }

// flamingoResolveDependencies copies Flamingo's config.resolveDependencies,
// which expands Depends before dingo sees the modules.
func flamingoResolveDependencies(modules []dingo.Module, known map[any]struct{}) []dingo.Module {
	final := make([]dingo.Module, 0, len(modules))

	if known == nil {
		known = make(map[any]struct{})
	}

	for _, module := range modules {
		var key any = reflect.TypeOf(module)
		if key == reflect.TypeOf(dingo.ModuleFunc(nil)) {
			key = reflect.ValueOf(module)
		}

		if _, ok := known[key]; ok {
			continue
		}

		known[key] = struct{}{}

		if depender, ok := module.(dingo.Depender); ok {
			final = append(final, flamingoResolveDependencies(depender.Depends(), known)...)
		}

		final = append(final, module)
	}

	return final
}

// probeModuleSelfDependency uses a module that depends on itself, directly
// and through another module.
func probeModuleSelfDependency(r *recorder) {
	r.add("NewInjector", catch(func() string { _, err := dingo.NewInjector(new(selfModule)); return errText(err) }))
	r.add("configured", takeRecorded())
	r.add("nested", catch(func() string { _, err := dingo.NewInjector(new(outerModule)); return errText(err) }))
	r.add("TryModule", catch(func() string { return errText(dingo.TryModule(new(selfModule))) }))
	r.add("child.InitModules", catch(func() string {
		root, _ := dingo.NewInjector()
		child, _ := root.Child()

		return errText(child.InitModules(new(selfModule)))
	}))
}

// probeModuleCycle uses two modules that depend on each other.
func probeModuleCycle(r *recorder) {
	r.add("NewInjector", catch(func() string { _, err := dingo.NewInjector(new(cycleModuleA)); return errText(err) }))
	r.add("TryModule", catch(func() string { return errText(dingo.TryModule(new(cycleModuleA))) }))
}

// probeModuleConfigureOrder records the Configure order for a module graph,
// as given and pre-expanded the way Flamingo does it, and how repeated
// modules and ModuleFuncs are deduplicated.
func probeModuleConfigureOrder(r *recorder) {
	modules := []dingo.Module{new(otherModule), new(topModule), new(baseModule)}

	_, err := dingo.NewInjector(modules...)
	r.add("as-given", takeRecorded()+" "+errText(err))

	expanded := flamingoResolveDependencies(modules, nil)
	names := make([]string, 0, len(expanded))

	for _, m := range expanded {
		names = append(names, reflect.TypeOf(m).Elem().Name())
	}

	_, err = dingo.NewInjector(expanded...)
	r.add("flamingo-expanded", strings.Join(names, ",")+" -> "+takeRecorded()+" "+errText(err))

	f1 := dingo.ModuleFunc(func(*dingo.Injector) { record("f1") })
	_, err = dingo.NewInjector(f1, new(topModule), f1, dingo.ModuleFunc(func(*dingo.Injector) { record("f2") }), new(topModule))
	r.add("dedup", takeRecorded()+" "+errText(err))
}

type configuredModule struct {
	Site string `inject:"config:site"`
}

func (m *configuredModule) Configure(*dingo.Injector) { record("site=" + m.Site) }

// probeModuleFieldInjectionFromAreaConfig injects area config into module
// fields before Configure, in the root and child areas.
func probeModuleFieldInjectionFromAreaConfig(r *recorder) {
	root := mustArea(nil, map[string]any{"site": "root"}, new(configuredModule))
	mustArea(root, map[string]any{"site": "fr"}, new(configuredModule))
	mustArea(root, map[string]any{"other": "x"}, new(configuredModule))
	r.add("configured", takeRecorded())
}

// probeModuleTryModule follows Flamingo's config.TryModules: a module binding
// config instances plus the modules under test.
func probeModuleTryModule(r *recorder) {
	r.add("ok", catch(func() string {
		return errText(dingo.TryModule(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(string)).AnnotatedWith("config:site").ToInstance("x")
		}), new(configuredModule)))
	}))
	r.add("missing-config", catch(func() string { return errText(dingo.TryModule(new(configuredModule))) }))
	r.add("panic-in-configure", catch(func() string {
		return errText(dingo.TryModule(dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(nil) })))
	}))
	r.add("eager-not-built", catch(func() string {
		before := constructions.Load()
		err := dingo.TryModule(dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(countedHandler)).AsEagerSingleton() }))

		return errText(err) + fmt.Sprintf(" constructed=%d", constructions.Load()-before)
	}))
}
