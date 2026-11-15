package releasediff_test

import (
	"fmt"
	"reflect"
	"sort"

	"flamingo.me/dingo"
)

// Probes on how dingo normalises the types it is given: pointer or value,
// interface or struct, Go values or reflect.Type arguments.

func init() {
	register("types_get_instance_forms", probeTypesGetInstanceForms)
	register("types_bind_forms", probeTypesBindForms)
	register("types_value_vs_pointer_instances", probeTypesValueVsPointerInstances)
	register("types_interface_targets", probeTypesInterfaceTargets)
	register("types_multi_and_map_pointer_forms", probeTypesMultiAndMapPointerForms)
}

// probeTypesGetInstanceForms binds Target and Handler once and asks for them
// in every form GetInstance accepts.
func probeTypesGetInstanceForms(r *recorder) {
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Target)).ToInstance(&Target{"bound"})
		injector.Bind(new(Handler)).ToInstance(&namedHandler{"bound"})
	}))
	r.add("init", errText(err))

	forms := []struct {
		label string
		of    any
	}{
		{"Target:new(Target)", new(Target)},
		{"Target:nil-pointer", (*Target)(nil)},
		{"Target:value", Target{}},
		{"Target:new(*Target)", new(*Target)},
		{"Target:reflect:Target", reflect.TypeOf(Target{})},
		{"Target:reflect:*Target", pointerToTarget},
		{"Target:reflect:**Target", pointerToPointerTarget},
		{"Handler:new(Handler)", new(Handler)},
		{"Handler:nil-pointer", (*Handler)(nil)},
		{"Handler:reflect:Handler", reflect.TypeOf((*Handler)(nil)).Elem()},
		{"Handler:reflect:*Handler", reflect.TypeOf((*Handler)(nil))},
		{"Handler:reflect:**Handler", reflect.TypeOf((**Handler)(nil))},
	}
	for _, form := range forms {
		r.add(form.label, catch(func() string { return result(injector.GetInstance(form.of)) }))
	}
}

// probeTypesBindForms binds Target in different forms, each in its own
// injector, and reads it back.
func probeTypesBindForms(r *recorder) {
	forms := []struct {
		label string
		what  any
	}{
		{"value", Target{}},
		{"pointer", &Target{}},
		{"nil-pointer", (*Target)(nil)},
		{"reflect.Type", reflect.TypeOf(Target{})},
	}
	for _, form := range forms {
		r.add(form.label, catch(func() string {
			injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
				injector.Bind(form.what).ToInstance(&Target{"bound"})
			}))
			if err != nil {
				return errText(err)
			}

			return result(injector.GetInstance(new(Target)))
		}))
	}

	r.add("to:pointer-pointer", catch(func() string {
		injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).To(new(*namedHandler))
		}))

		return result(injector.GetInstance(new(Handler)))
	}))
	r.add("to:same-type", catch(func() string {
		injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Target)).To(Target{})
		}))

		return result(injector.GetInstance(new(Target)))
	}))
}

type valueTargetHolder struct {
	T Target `inject:""`
}

// probeTypesValueVsPointerInstances binds Target to a value instance, a value
// provider and a pointer provider, and reads it by GetInstance and *Target
// and Target fields.
func probeTypesValueVsPointerInstances(r *recorder) {
	bindings := []struct {
		label string
		bind  func(*dingo.Binding)
	}{
		{"value-instance", func(b *dingo.Binding) { b.ToInstance(Target{"value"}) }},
		{"value-provider", func(b *dingo.Binding) { b.ToProvider(func() Target { return Target{"value"} }) }},
		{"pointer-provider", func(b *dingo.Binding) { b.ToProvider(func() *Target { return &Target{"pointer"} }) }},
	}
	for _, binding := range bindings {
		injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			binding.bind(injector.Bind(new(Target)))
		}))
		r.add(binding.label+".init", errText(err))
		r.add(binding.label+".GetInstance", catch(func() string { return result(injector.GetInstance(new(Target))) }))
		r.add(binding.label+".field(*Target)", catch(func() string {
			var h targetHolder

			err := injector.RequestInjection(&h)

			return errText(err) + " " + show(h.T)
		}))
		r.add(binding.label+".field(Target)", catchErr(func() error { var h valueTargetHolder; return injector.RequestInjection(&h) }))
	}
}

// probeTypesInterfaceTargets binds interfaces to implementations given in
// different forms, and to another interface.
func probeTypesInterfaceTargets(r *recorder) {
	r.add("to-value-of-pointer-receiver-type", catch(func() string {
		injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).To(namedHandler{})
		}))

		return result(injector.GetInstance(new(Handler)))
	}))
	r.add("toinstance-value-of-pointer-receiver-type", catch(func() string {
		injector, _ := dingo.NewInjector()
		injector.Bind(new(Handler)).ToInstance(namedHandler{"value"})

		return "ok"
	}))
	r.add("to-unbound-interface", catch(func() string {
		injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).To(new(Service))
		}))

		return result(injector.GetInstance(new(Handler)))
	}))
	r.add("to-bound-interface", catch(func() string {
		injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).To(new(Service))
			injector.Bind(new(Service)).To(realService{})
		}))

		return result(injector.GetInstance(new(Handler)))
	}))
	r.add("provider-returns-implementation", catch(func() string {
		injector, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).ToProvider(func() *namedHandler { return &namedHandler{"from-provider"} })
		}))

		return result(injector.GetInstance(new(Handler)))
	}))
}

// probeTypesMultiAndMapPointerForms binds multibindings and map keys for
// Target and *Target and reads them as slices and maps of both.
func probeTypesMultiAndMapPointerForms(r *recorder) {
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMulti(new(Target)).ToInstance(&Target{"multi-value-type"})
		injector.BindMulti(new(*Target)).ToInstance(&Target{"multi-pointer-type"})
		injector.BindMap(new(Target), "value-type").ToInstance(&Target{"map-value-type"})
		injector.BindMap(new(*Target), "pointer-type").ToInstance(&Target{"map-pointer-type"})
	}))
	r.add("init", errText(err))

	reads := []struct {
		label string
		of    any
	}{
		{"[]*Target", new([]*Target)},
		{"[]Target", new([]Target)},
		{"map[string]*Target", new(map[string]*Target)},
		{"map[string]Target", new(map[string]Target)},
	}
	for _, read := range reads {
		r.add(read.label, catch(func() string {
			v, err := injector.GetInstance(read.of)
			if err != nil {
				return errText(err)
			}

			return targets(v)
		}))
	}

	r.add("map:pointer-type", catch(func() string { return result(injector.GetAnnotatedInstance(new(Target), "map:pointer-type")) }))
	r.add("map:pointer-type-as-reflect:**Target", catch(func() string {
		return result(injector.GetAnnotatedInstance(pointerToPointerTarget, "map:pointer-type"))
	}))
}

// targets prints a slice or map of Target or *Target in a stable order.
func targets(v any) string {
	value := reflect.ValueOf(v)

	var parts []string

	if value.Kind() == reflect.Slice {
		for i := range value.Len() {
			parts = append(parts, show(value.Index(i).Interface()))
		}
	} else {
		for _, key := range value.MapKeys() {
			parts = append(parts, key.String()+":"+show(value.MapIndex(key).Interface()))
		}

		sort.Strings(parts)
	}

	return fmt.Sprintf("%T%v", v, parts)
}
