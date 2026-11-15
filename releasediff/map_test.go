package releasediff_test

import (
	"fmt"

	"flamingo.me/dingo"
)

// Probes on map bindings (BindMap), whole maps and single keys read through
// a "map:<key>" annotation, mostly across areas.

func init() {
	register("map_whole_per_area", probeMapWholePerArea)
	register("map_key_per_area", probeMapKeyPerArea)
	register("map_key_field_in_child", probeMapKeyFieldInChild)
	register("map_key_optional_in_child", probeMapKeyOptionalInChild)
	register("map_key_factory_in_child", probeMapKeyFactoryInChild)
	register("map_key_scopes_in_child", probeMapKeyScopesInChild)
	register("map_key_regular_binding_precedence", probeMapKeyRegularBindingPrecedence)
	register("map_provider_values_per_area", probeMapProviderValuesPerArea)
	register("map_annotated", probeMapAnnotated)
	register("map_key_slice_map_provider_fields", probeMapKeySliceMapProviderFields)
}

// probeMapWholePerArea injects the whole map in each area: parent keys
// first, overridden by the area's own.
func probeMapWholePerArea(r *recorder) {
	for _, a := range mapAreas() {
		r.add(a.name, handlerNames(a.injector.GetInstance(new(map[string]Handler))))
	}
}

// probeMapKeyPerArea reads single keys with GetAnnotatedInstance in each area.
func probeMapKeyPerArea(r *recorder) {
	for _, a := range mapAreas() {
		for _, key := range []string{"a", "b", "c", "site", "zzz"} {
			r.add(a.name+".map:"+key, catch(func() string { return result(a.injector.GetAnnotatedInstance(new(Handler), "map:"+key)) }))
		}
	}
}

type mapKeyHolder struct {
	H Handler `inject:"map:a"`
}

// probeMapKeyFieldInChild injects a key bound in the root into a field, in a
// child that binds no map keys (fr) and one that binds other keys (de).
func probeMapKeyFieldInChild(r *recorder) {
	areas := mapAreas()
	de, fr := areas[1].injector, areas[2].injector

	var h mapKeyHolder

	r.add("fr.inject", catchErr(func() error { return fr.RequestInjection(&h) }))
	r.add("fr.field", show(h.H))
	r.add("fr.GetInstance(holder)", catch(func() string {
		v, err := fr.GetInstance(new(mapKeyHolder))
		if err != nil {
			return errText(err)
		}

		return show(v.(*mapKeyHolder).H)
	}))

	var h2 mapKeyHolder

	r.add("de.inject", catchErr(func() error { return de.RequestInjection(&h2) }))
	r.add("de.field", show(h2.H))
}

type mapKeyOptionalHolder struct {
	H       Handler `inject:"map:a,optional"`
	T       *Target `inject:"map:t,optional"`
	Missing Handler `inject:"map:zzz,optional"`
}

// probeMapKeyOptionalInChild injects optional map keys bound in the root,
// in the root and in a child.
func probeMapKeyOptionalInChild(r *recorder) {
	root := mustArea(nil, map[string]any{"site": "root"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Handler), "a").ToInstance(&namedHandler{"root-a"})
		injector.BindMap(new(Target), "t").ToInstance(&Target{N: "root-t"})
	}))
	fr := mustArea(root, map[string]any{"site": "fr"})

	for _, a := range []namedArea{{"root", root}, {"fr", fr}} {
		var h mapKeyOptionalHolder

		r.add(a.name+".inject", catchErr(func() error { return a.injector.RequestInjection(&h) }))
		r.add(a.name+".H", show(h.H))
		r.add(a.name+".T", show(h.T))
		r.add(a.name+".Missing", show(h.Missing))
	}
}

// Factory mirrors Flamingo's auth.RequestIdentifierFactory, a func type bound
// with BindMap.
type Factory func(cfg map[string]any) (Handler, error)

type oidcFactoryHolder struct {
	OIDC Factory `inject:"map:oidc"`
}

// probeMapKeyFactoryInChild follows core/auth/example/custom/oidc.go: a
// provider in one module injects another module's factory via
// inject:"map:oidc". The base factory is bound in the root area and the
// custom one in a child; then the common case with both in one injector.
func probeMapKeyFactoryInChild(r *recorder) {
	custom := func(c *oidcFactoryHolder) Factory {
		return func(cfg map[string]any) (Handler, error) {
			h, err := c.OIDC(cfg)
			if err != nil {
				return nil, err
			}

			return &namedHandler{"custom(" + h.Name() + ")"}, nil
		}
	}
	oidc := Factory(func(cfg map[string]any) (Handler, error) {
		return &namedHandler{fmt.Sprint("oidc:", cfg["x"])}, nil
	})
	read := func(injector *dingo.Injector) string {
		return catch(func() string {
			v, err := injector.GetInstance(new(map[string]Factory))
			if err != nil {
				return errText(err)
			}

			factories := v.(map[string]Factory)
			h, err := factories["custom"](map[string]any{"x": 1})

			return fmt.Sprintf("len=%d custom=%s", len(factories), result(h, err))
		})
	}

	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Factory), "oidc").ToInstance(oidc)
	}))
	child, err := newArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Factory), "custom").ToProvider(custom)
	}))

	r.add("child.init", errText(err))
	r.add("child.map", read(child))

	single := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Factory), "oidc").ToInstance(oidc)
		injector.BindMap(new(Factory), "custom").ToProvider(custom)
	}))
	r.add("single.map", read(single))
}

// probeMapKeyScopesInChild reads scoped map keys bound in the root from a
// child, and compares the instances.
func probeMapKeyScopesInChild(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Handler), "single").To(countedHandler{}).In(dingo.Singleton)
		injector.BindMap(new(Handler), "childsingle").To(countedHandler{}).In(dingo.ChildSingleton)
		injector.BindMap(new(Handler), "plain").To(countedHandler{})
	}))
	fr := mustArea(root, nil)

	for _, key := range []string{"single", "childsingle", "plain"} {
		get := func(injector *dingo.Injector) (any, string) {
			var v any

			s := catch(func() string {
				var err error

				v, err = injector.GetAnnotatedInstance(new(Handler), "map:"+key)

				return result(v, err)
			})

			return v, s
		}
		inRoot, s1 := get(root)
		inChild1, s2 := get(fr)
		inChild2, s3 := get(fr)
		r.add(key, fmt.Sprintf("root=%s fr1=%s fr2=%s fr1-fr2=%s root-fr=%s",
			s1, s2, s3, identity(inChild1, inChild2), identity(inRoot, inChild1)))
	}

	r.add("whole-map-vs-key", catch(func() string {
		m, err := fr.GetInstance(new(map[string]Handler))
		if err != nil {
			return errText(err)
		}

		k, err := fr.GetAnnotatedInstance(new(Handler), "map:single")
		if err != nil {
			return errText(err)
		}

		return "single:" + identity(m.(map[string]Handler)["single"], k)
	}))
}

// probeMapKeyRegularBindingPrecedence binds a regular binding annotated
// "map:<key>" next to a map key of the same name, in root and child.
func probeMapKeyRegularBindingPrecedence(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).AnnotatedWith("map:p").ToInstance(&namedHandler{"root-regular-p"})
		injector.BindMap(new(Handler), "p").ToInstance(&namedHandler{"root-map-p"})
		injector.BindMap(new(Handler), "q").ToInstance(&namedHandler{"root-map-q"})
	}))
	de := mustArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Handler), "p").ToInstance(&namedHandler{"de-map-p"})
		injector.Bind(new(Handler)).AnnotatedWith("map:q").ToInstance(&namedHandler{"de-regular-q"})
	}))
	fr := mustArea(root, nil)

	for _, a := range []namedArea{{"root", root}, {"de", de}, {"fr", fr}} {
		r.add(a.name+".map:p", catch(func() string { return result(a.injector.GetAnnotatedInstance(new(Handler), "map:p")) }))
		r.add(a.name+".map:q", catch(func() string { return result(a.injector.GetAnnotatedInstance(new(Handler), "map:q")) }))
		r.add(a.name+".map", handlerNames(a.injector.GetInstance(new(map[string]Handler))))
	}
}

// probeMapProviderValuesPerArea injects a map of providers in two areas.
func probeMapProviderValuesPerArea(r *recorder) {
	areas := mapAreas()
	for _, a := range areas[1:3] {
		r.add(a.name, catch(func() string {
			v, err := a.injector.GetInstance(new(map[string]HandlerProvider))
			if err != nil {
				return errText(err)
			}

			handlers := map[string]Handler{}
			for k, p := range v.(map[string]HandlerProvider) {
				handlers[k] = p()
			}

			return handlerNames(handlers, nil)
		}))
	}
}

type annotatedMapHolder struct {
	Plain     map[string]Handler `inject:""`
	Annotated map[string]Handler `inject:"ann"`
}

// probeMapAnnotated binds map keys with AnnotatedWith, which filters whole
// map injection by annotation.
func probeMapAnnotated(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Handler), "x").ToInstance(&namedHandler{"plain-x"})
		injector.BindMap(new(Handler), "y").AnnotatedWith("ann").ToInstance(&namedHandler{"ann-y"})
	}))
	child := mustArea(root, nil)

	for _, a := range []namedArea{{"root", root}, {"child", child}} {
		var h annotatedMapHolder

		err := catchErr(func() error { return a.injector.RequestInjection(&h) })
		r.add(a.name, err+" plain="+handlerNames(h.Plain, nil)+" ann="+handlerNames(h.Annotated, nil))
		r.add(a.name+".map:y", catch(func() string { return result(a.injector.GetAnnotatedInstance(new(Handler), "map:y")) }))
	}
}

type StringMap map[string]any

// probeMapKeySliceMapProviderFields injects required "map:<key>" fields whose
// type is a slice, a map or a provider. A missing key does not fail there but
// falls through to multibinding, map or provider resolution.
func probeMapKeySliceMapProviderFields(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new([]string), "k").ToInstance([]string{"root-slice"})
		injector.BindMap(new(StringMap), "k").ToInstance(StringMap{"from": "root"})
		injector.BindMap(new(Handler), "a").ToInstance(&namedHandler{"root-a"})
	}))
	fr := mustArea(root, nil)

	for _, a := range []namedArea{{"root", root}, {"fr", fr}} {
		var h struct {
			S []string        `inject:"map:k"`
			M StringMap       `inject:"map:k"`
			P HandlerProvider `inject:"map:a"`
		}

		r.add(a.name+".inject", catchErr(func() error { return a.injector.RequestInjection(&h) }))
		r.add(a.name+".S", fmt.Sprintf("%#v", h.S))
		r.add(a.name+".M", fmt.Sprintf("%#v", h.M))
		r.add(a.name+".P()", catch(func() string {
			if h.P == nil {
				return "nil func"
			}

			return show(h.P())
		}))
	}
}
