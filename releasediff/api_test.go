package releasediff_test

import (
	"fmt"
	"reflect"
	"strings"

	"flamingo.me/dingo"
)

// Probes on public API corners not covered by the area, map and override
// probes: scope instances, custom scopes, Inspect, interceptor reach and
// calling InitModules more than once.

func init() {
	register("scope_instances", probeScopeInstances)
	register("scope_custom", probeScopeCustom)
	register("inspect", probeInspect)
	register("interceptor_reach", probeInterceptorReach)
	register("init_modules_twice", probeInitModulesTwice)
}

type (
	freshScopeService  struct{ countedHandler }
	globalScopeService struct{ countedHandler }
	globalChildService struct{ countedHandler }
)

// probeScopeInstances checks which scope instance a binding uses. Scopes are
// looked up by type, so In(dingo.NewSingletonScope()) uses the injector's
// Singleton; dingo.Singleton and dingo.ChildSingleton are shared by
// unrelated root injectors.
func probeScopeInstances(r *recorder) {
	module := dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(freshScopeService)).In(dingo.NewSingletonScope())
		injector.Bind(new(globalScopeService)).In(dingo.Singleton)
		injector.Bind(new(globalChildService)).In(dingo.ChildSingleton)
	})
	first, _ := dingo.NewInjector(module)
	second, _ := dingo.NewInjector(module)
	get := func(injector *dingo.Injector, of any) any {
		v, _ := injector.GetInstance(of)

		return v
	}

	for _, of := range []any{new(freshScopeService), new(globalScopeService), new(globalChildService)} {
		label := reflect.TypeOf(of).Elem().Name()
		r.add(label, fmt.Sprintf("twice=%s across-roots=%s",
			identity(get(first, of), get(first, of)), identity(get(first, of), get(second, of))))
	}
}

// recordingScope records what dingo passes to a scope and builds a new
// instance each time.
type recordingScope struct{ calls []string }

func (s *recordingScope) ResolveType(
	t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error),
) (reflect.Value, error) {
	s.calls = append(s.calls, fmt.Sprintf("%s@%q", t, annotation))

	return unscoped(t, annotation, false)
}

// probeScopeCustom registers a custom scope with BindScope and records the
// types and annotations it is asked for.
func probeScopeCustom(r *recorder) {
	scope := &recordingScope{}
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindScope(scope)
		injector.Bind(new(Handler)).To(namedHandler{}).In(&recordingScope{})
		injector.Bind(new(Target)).AnnotatedWith("a").In(&recordingScope{})
	}))
	r.add("init", errText(err))
	r.add("Handler", catch(func() string { return result(injector.GetInstance(new(Handler))) }))
	r.add("Target@a", catch(func() string { return result(injector.GetAnnotatedInstance(new(Target), "a")) }))
	r.add("calls", strings.Join(scope.calls, ","))

	child, _ := injector.Child()

	r.add("child.Handler", catch(func() string { return result(child.GetInstance(new(Handler))) }))
	r.add("calls-after-child", strings.Join(scope.calls, ","))
}

// probeInspect lists what Inspect reports for an injector with plain,
// annotated, scoped, provider, multi and map bindings and overrides, and for
// its child.
func probeInspect(r *recorder) {
	root, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).To(namedHandler{}).In(dingo.Singleton)
		injector.Bind(new(Handler)).AnnotatedWith("a").ToProvider(func() Handler { return nil })
		injector.Bind(new(Target)).ToInstance(&Target{"orig"})
		injector.Override(new(*Target), "").ToInstance(&Target{"over"})
		injector.Bind(new(Service)).To(realService{})
		injector.Override(new(Service), "").To(fakeService{})
		injector.BindMulti(new(Handler)).ToInstance(&namedHandler{"m"})
		injector.BindMap(new(Handler), "k").AnnotatedWith("ann").To(namedHandler{})
	}))
	r.add("init", errText(err))

	child, _ := root.Child()

	for _, a := range []namedArea{{"root", root}, {"child", child}} {
		for label, value := range inspect(a.injector) {
			r.add(a.name+"."+label, value)
		}
	}
}

// inspect turns Inspect callbacks into labelled values. Bindings of the same
// type and annotation are numbered in the order Inspect reports them.
func inspect(injector *dingo.Injector) map[string]string {
	found := map[string]string{}
	add := func(kind string, of reflect.Type, key, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
		base := strings.ReplaceAll(fmt.Sprintf("%s:%s:%s@%s", kind, of, key, annotation), " ", "_")

		label := base
		for n := 2; found[label] != ""; n++ {
			label = fmt.Sprintf("%s#%d", base, n)
		}

		value := fmt.Sprintf("to=%v provider=%v scope=%T", to, provider != nil, in)
		if instance != nil {
			value += fmt.Sprintf(" instance=%s", instance.Type())
			if t, ok := instance.Interface().(*Target); ok {
				value += "{" + t.N + "}"
			}
		}

		found[label] = value
	}

	injector.Inspect(dingo.Inspector{
		InspectBinding: func(of reflect.Type, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			add("binding", of, "", annotation, to, provider, instance, in)
		},
		InspectMultiBinding: func(of reflect.Type, index int, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			add("multi", of, fmt.Sprint(index), annotation, to, provider, instance, in)
		},
		InspectMapBinding: func(of reflect.Type, key string, annotation string, to reflect.Type, provider, instance *reflect.Value, in dingo.Scope) {
			add("map", of, key, annotation, to, provider, instance, in)
		},
		InspectParent: func(*dingo.Injector) { found["parent"] = "reported" },
	})

	return found
}

// probeInterceptorReach checks which resolutions an interceptor wraps: plain,
// annotated, scoped, through a provider, multibindings and map keys.
func probeInterceptorReach(r *recorder) {
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).ToInstance(&namedHandler{"plain"})
		injector.Bind(new(Handler)).AnnotatedWith("a").ToInstance(&namedHandler{"annotated"})
		injector.Bind(new(Handler)).AnnotatedWith("s").To(namedHandler{}).In(dingo.Singleton)
		injector.BindMulti(new(Handler)).ToInstance(&namedHandler{"multi"})
		injector.BindMap(new(Handler), "k").ToInstance(&namedHandler{"map"})
		injector.BindInterceptor(new(Handler), childInterceptor{})
	}))
	r.add("init", errText(err))
	r.add("plain", catch(func() string { return result(injector.GetInstance(new(Handler))) }))
	r.add("annotated", catch(func() string { return result(injector.GetAnnotatedInstance(new(Handler), "a")) }))
	r.add("singleton", catch(func() string {
		first, _ := injector.GetAnnotatedInstance(new(Handler), "s")
		second, err := injector.GetAnnotatedInstance(new(Handler), "s")

		return result(second, err) + " twice=" + identity(first, second)
	}))
	r.add("provider", catch(func() string {
		v, err := injector.GetInstance(new(HandlerProvider))
		if err != nil {
			return errText(err)
		}

		return show(v.(HandlerProvider)())
	}))
	r.add("multi", handlerNames(injector.GetInstance(new([]Handler))))
	r.add("map", handlerNames(injector.GetInstance(new(map[string]Handler))))
	r.add("map:k", catch(func() string { return result(injector.GetAnnotatedInstance(new(Handler), "map:k")) }))
}

// probeInitModulesTwice calls InitModules again on an initialised injector.
// Overrides from the first call are evaluated again.
func probeInitModulesTwice(r *recorder) {
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Target)).ToInstance(&Target{"orig"})
		injector.Override(new(Target), "").ToInstance(&Target{"over"})
	}))
	r.add("first", errText(err))

	err = injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Target)).AnnotatedWith("second").ToInstance(&Target{"second"})
		injector.Bind(new(Service)).To(realService{})
	}))
	r.add("second", errText(err))
	r.add("Target", catch(func() string { return result(injector.GetInstance(new(Target))) }))
	r.add("Target@second", catch(func() string { return result(injector.GetAnnotatedInstance(new(Target), "second")) }))

	err = injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Service)).To(fakeService{})
	}))
	r.add("third-rebinds", errText(err))
}
