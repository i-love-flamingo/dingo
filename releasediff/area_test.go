package releasediff_test

import (
	"fmt"
	"sync"
	"sync/atomic"

	"flamingo.me/dingo"
)

// Probes on Flamingo's area tree: a root injector, one child per area, config
// bound per area.

func init() {
	register("area_config_injection", probeAreaConfigInjection)
	register("area_parent_binding_with_child_config", probeAreaParentBindingWithChildConfig)
	register("area_scopes_identity", probeAreaScopesIdentity)
	register("area_eager_singletons", probeAreaEagerSingletons)
	register("area_multibinding_order", probeAreaMultibindingOrder)
	register("area_interceptors", probeAreaInterceptors)
	register("area_concurrent_singleton", probeAreaConcurrentSingleton)
}

type configHolder struct {
	Site     string  `inject:"config:site"`
	Root     string  `inject:"config:only.root"`
	NumInt   int     `inject:"config:num"`
	NumFloat float64 `inject:"config:num"`
	Missing  string  `inject:"config:missing,optional"`
	MissingB bool    `inject:"config:missingb,optional"`
}

// probeAreaConfigInjection injects config values in each area: own values,
// inherited values, the int and float forms of a number, optional misses.
func probeAreaConfigInjection(r *recorder) {
	areas := mapAreas()
	for _, a := range areas {
		r.add(a.name, catch(func() string {
			var h configHolder
			if err := a.injector.RequestInjection(&h); err != nil {
				return errText(err)
			}

			return fmt.Sprintf("%+v", h)
		}))
	}

	r.add("missing-required", catch(func() string {
		var h struct {
			X string `inject:"config:nope"`
		}

		return errText(areas[2].injector.RequestInjection(&h))
	}))
}

// probeAreaParentBindingWithChildConfig resolves a binding made in the root
// area from child areas. An unscoped or child-singleton binding is built with
// the asking area's config, a singleton with the config of the first area
// that asks.
func probeAreaParentBindingWithChildConfig(r *recorder) {
	root := mustArea(nil, map[string]any{"site": "root"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).To(siteHandler{})
		injector.Bind(new(Handler)).AnnotatedWith("single").To(siteHandler{}).In(dingo.Singleton)
		injector.Bind(new(Handler)).AnnotatedWith("childsingle").To(siteHandler{}).In(dingo.ChildSingleton)
	}))
	de := mustArea(root, map[string]any{"site": "de"})
	fr := mustArea(root, map[string]any{"site": "fr"})

	for _, annotation := range []string{"", "single", "childsingle"} {
		get := func(injector *dingo.Injector) string {
			return catch(func() string { return result(injector.GetAnnotatedInstance(new(Handler), annotation)) })
		}
		r.add("annotation:"+annotation, fmt.Sprintf("de=%s fr=%s root=%s", get(de), get(fr), get(root)))
	}
}

type (
	childSingletonService struct{ countedHandler }
	singletonService      struct{ countedHandler }
)

// probeAreaScopesIdentity compares instances across areas for Singleton,
// ChildSingleton and unscoped bindings.
func probeAreaScopesIdentity(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(childSingletonService)).In(dingo.ChildSingleton)
		injector.Bind(new(singletonService)).In(dingo.Singleton)
	}))
	de := mustArea(root, nil)
	fr := mustArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Leaf)).In(dingo.ChildSingleton)
	}))
	get := func(injector *dingo.Injector, of any) any {
		v, _ := injector.GetInstance(of)

		return v
	}
	childS, single := new(childSingletonService), new(singletonService)

	r.add("childsingleton", fmt.Sprintf("root-twice=%s de-twice=%s root-de=%s de-fr=%s",
		identity(get(root, childS), get(root, childS)), identity(get(de, childS), get(de, childS)),
		identity(get(root, childS), get(de, childS)), identity(get(de, childS), get(fr, childS))))
	r.add("singleton", fmt.Sprintf("root-de=%s de-fr=%s",
		identity(get(root, single), get(de, single)), identity(get(de, single), get(fr, single))))
	r.add("unscoped", identity(get(root, new(Leaf)), get(root, new(Leaf))))
	r.add("child-own-childsingleton", identity(get(fr, new(Leaf)), get(fr, new(Leaf))))
	r.add("child-own-vs-root", identity(get(root, new(Leaf)), get(fr, new(Leaf))))
}

var eagerBuilds atomic.Int64

type eagerService struct{}

func (*eagerService) Inject() { eagerBuilds.Add(1) }

// probeAreaEagerSingletons follows Flamingo's eager singleton handling: areas
// switch eager building off and build later, children with includeParent.
func probeAreaEagerSingletons(r *recorder) {
	module := dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(eagerService)).AsEagerSingleton() })
	root := mustArea(nil, nil, module)

	r.add("after-area-init", fmt.Sprint(eagerBuilds.Load()))

	fr := mustArea(root, nil)
	r.add("child.Build(false)", errText(fr.BuildEagerSingletons(false))+fmt.Sprintf(" built=%d", eagerBuilds.Load()))
	r.add("child.Build(true)", errText(fr.BuildEagerSingletons(true))+fmt.Sprintf(" built=%d", eagerBuilds.Load()))
	r.add("root.Build(false)", errText(root.BuildEagerSingletons(false))+fmt.Sprintf(" built=%d", eagerBuilds.Load()))

	_, err := dingo.NewInjector(module)
	r.add("NewInjector", errText(err)+fmt.Sprintf(" built=%d", eagerBuilds.Load()))

	_, err = dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Service)).To(realService{}).AsEagerSingleton()
		injector.Bind(new(Handler)).ToProvider(func(s Service) Handler { return &namedHandler{"p:" + s.Name()} }).AsEagerSingleton()
	}))
	r.add("interface-and-provider", errText(err))

	_, err = dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).AnnotatedWith("x").To(siteHandler{}).AsEagerSingleton()
	}))
	r.add("missing-config", errText(err))
}

// probeAreaMultibindingOrder reads multibindings per area: parent entries
// first, then the area's own; also as providers, empty, and annotated.
func probeAreaMultibindingOrder(r *recorder) {
	areas := mapAreas()
	for _, a := range areas {
		r.add(a.name, handlerNames(a.injector.GetInstance(new([]Handler))))
	}

	r.add("de.providers", catch(func() string {
		v, err := areas[1].injector.GetInstance(new([]HandlerProvider))
		if err != nil {
			return result(nil, err)
		}

		var handlers []Handler
		for _, p := range v.([]HandlerProvider) {
			handlers = append(handlers, p())
		}

		return handlerNames(handlers, nil)
	}))
	r.add("unbound", catch(func() string { return result(areas[2].injector.GetInstance(new([]Service))) }))
	r.add("annotated", catch(func() string {
		root, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.BindMulti(new(Handler)).AnnotatedWith("x").ToInstance(&namedHandler{"x1"})
			injector.BindMulti(new(Handler)).ToInstance(&namedHandler{"plain"})
		}))
		child, _ := root.Child()
		child.BindMulti(new(Handler)).AnnotatedWith("x").ToInstance(&namedHandler{"x2-child"})
		_ = child.InitModules()

		var h struct {
			X     []Handler `inject:"x"`
			Plain []Handler `inject:""`
		}

		err := child.RequestInjection(&h)

		return errText(err) + " x=" + handlerNames(h.X, nil) + " plain=" + handlerNames(h.Plain, nil)
	}))
}

type siteInterceptor struct {
	Handler
	Site string `inject:"config:site"`
}

func (i *siteInterceptor) Name() string { return "intercept[" + i.Site + "](" + i.Handler.Name() + ")" }

type childInterceptor struct{ Handler }

func (i *childInterceptor) Name() string { return "child(" + i.Handler.Name() + ")" }

// probeAreaInterceptors stacks interceptors bound in the root and a child area.
func probeAreaInterceptors(r *recorder) {
	root := mustArea(nil, map[string]any{"site": "root"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).ToInstance(&namedHandler{"base"})
		injector.BindInterceptor(new(Handler), siteInterceptor{})
	}))
	de := mustArea(root, map[string]any{"site": "de"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindInterceptor(new(Handler), childInterceptor{})
	}))
	fr := mustArea(root, map[string]any{"site": "fr"})

	for _, a := range []namedArea{{"root", root}, {"de", de}, {"fr", fr}} {
		r.add(a.name, catch(func() string { return result(a.injector.GetInstance(new(Handler))) }))
	}

	r.add("non-interface", catch(func() string { root.BindInterceptor(new(Leaf), siteInterceptor{}); return "ok" }))
}

// probeAreaConcurrentSingleton resolves one root singleton from the root and
// a child area at the same time; it must be built once.
func probeAreaConcurrentSingleton(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).To(countedHandler{}).In(dingo.Singleton)
	}))
	fr := mustArea(root, nil)
	before := constructions.Load()

	var wg sync.WaitGroup

	for n := range 64 {
		injector := root
		if n%2 == 1 {
			injector = fr
		}

		wg.Go(func() { _, _ = injector.GetInstance(new(Handler)) })
	}

	wg.Wait()
	r.add("constructions", fmt.Sprint(constructions.Load()-before))
}
