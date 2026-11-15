package releasediff_test

import (
	"fmt"
	"reflect"

	"flamingo.me/dingo"
)

// Probes on Override. Flamingo modules override bindings of modules they
// depend on, and Flamingo Commerce overrides adapters with fakes. Lookups
// resolve *T through the T binding, so the pointer cases matter: a change
// there once kept the original binding without an error.

func init() {
	register("override_interface_with_fake", probeOverrideInterfaceWithFake)
	register("override_module_order", probeOverrideModuleOrder)
	register("override_errors", probeOverrideErrors)
	register("override_twice", probeOverrideTwice)
	register("override_pointer_on_value_binding", probeOverridePointerOnValueBinding)
	register("override_pointer_on_pointer_binding", probeOverridePointerOnPointerBinding)
	register("override_pointer_with_value_and_pointer_bound", probeOverridePointerWithValueAndPointerBound)
	register("override_pointer_in_child", probeOverridePointerInChild)
	register("bind_pointer_type_only", probeBindPointerTypeOnly)
}

// pointerToTarget is the reflect.Type of *Target. GetInstance takes a
// reflect.Type as the type to resolve, after dropping one pointer level, so
// asking for **Target is the only way to reach a binding made for *Target.
var (
	pointerToTarget        = reflect.TypeOf((*Target)(nil))
	pointerToPointerTarget = reflect.TypeOf((**Target)(nil))
)

type targetHolder struct {
	T *Target `inject:""`
}

type annotatedTargetHolder struct {
	T *Target `inject:"a"`
}

// readTarget resolves Target the ways Flamingo code does and records each.
func readTarget(r *recorder, prefix string, injector *dingo.Injector, annotation string) {
	r.add(prefix+"GetInstance(Target)", catch(func() string { return result(injector.GetAnnotatedInstance(new(Target), annotation)) }))
	r.add(prefix+"GetInstance(reflect:**Target)", catch(func() string {
		return result(injector.GetAnnotatedInstance(pointerToPointerTarget, annotation))
	}))
	r.add(prefix+"field(*Target)", catch(func() string {
		if annotation == "" {
			var h targetHolder

			err := injector.RequestInjection(&h)

			return errText(err) + " " + show(h.T)
		}

		var h annotatedTargetHolder

		err := injector.RequestInjection(&h)

		return errText(err) + " " + show(h.T)
	}))
}

// probeOverrideInterfaceWithFake follows Flamingo Commerce product and
// category modules: bind the real adapter, then, with fakes enabled,
// Override((*I)(nil), "").To(fake{}).In(dingo.ChildSingleton) in the same
// module.
func probeOverrideInterfaceWithFake(r *recorder) {
	module := dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind((*Service)(nil)).To(realService{})
		injector.Override((*Service)(nil), "").To(fakeService{}).In(dingo.ChildSingleton)
	})
	root := mustArea(nil, nil, module)
	fr := mustArea(root, nil)
	inRoot1, _ := root.GetInstance((*Service)(nil))
	inRoot2, _ := root.GetInstance((*Service)(nil))
	inChild1, err := fr.GetInstance((*Service)(nil))
	inChild2, _ := fr.GetInstance((*Service)(nil))

	r.add("root", show(inRoot1)+" twice="+identity(inRoot1, inRoot2))
	r.add("fr", result(inChild1, err)+" twice="+identity(inChild1, inChild2)+" vs-root="+identity(inRoot1, inChild1))
}

// probeOverrideModuleOrder overrides from another module, listed before and
// after the binding module.
func probeOverrideModuleOrder(r *recorder) {
	override := dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Override(new(Service), "").To(fakeService{}) })
	bind := dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(Service)).To(realService{}) })

	injector, err := dingo.NewInjector(override, bind)
	r.add("override-first", errText(err)+" "+result(injector.GetInstance(new(Service))))

	injector, err = dingo.NewInjector(bind, override)
	r.add("bind-first", errText(err)+" "+result(injector.GetInstance(new(Service))))
}

// probeOverrideErrors covers overrides without a matching binding, annotated
// overrides and the (*T)(nil) form.
func probeOverrideErrors(r *recorder) {
	r.add("unknown", catch(func() string {
		_, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Override(new(Service), "").To(fakeService{}) }))

		return errText(err)
	}))
	r.add("only-multibinding", catch(func() string {
		_, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.BindMulti(new(Service)).To(realService{})
			injector.Override(new(Service), "").To(fakeService{})
		}))

		return errText(err)
	}))
	r.add("child-overrides-parent-binding", catch(func() string {
		root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(Service)).To(realService{}) }))
		_, err := newArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Override(new(Service), "").To(fakeService{}) }))

		return errText(err)
	}))
	r.add("annotated", catch(func() string {
		injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Service)).AnnotatedWith("x").To(realService{})
			injector.Bind(new(Service)).To(realService{})
			injector.Override(new(Service), "x").To(fakeService{})
		}))
		if err != nil {
			return errText(err)
		}

		annotated, _ := injector.GetAnnotatedInstance(new(Service), "x")
		plain, _ := injector.GetInstance(new(Service))

		return "x=" + show(annotated) + " plain=" + show(plain)
	}))
	r.add("nil-pointer-form", catch(func() string {
		injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind((*Target)(nil)).ToInstance(&Target{"orig"})
			injector.Override((*Target)(nil), "").ToInstance(&Target{"over"})
		}))
		if err != nil {
			return errText(err)
		}

		return result(injector.GetInstance(new(Target)))
	}))
}

// probeOverrideTwice overrides one binding from two modules; the later wins.
func probeOverrideTwice(r *recorder) {
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Target)).ToInstance(&Target{"orig"})
		injector.Override(new(Target), "").ToInstance(&Target{"first"})
		injector.Override(new(Target), "").ToInstance(&Target{"second"})
	}))

	r.add("init", errText(err))
	readTarget(r, "", injector, "")
}

// probeOverridePointerOnValueBinding binds Target and overrides *Target, plain
// and annotated. The override replaces the Target binding.
func probeOverridePointerOnValueBinding(r *recorder) {
	for _, annotation := range []string{"", "a"} {
		prefix := "plain."
		if annotation != "" {
			prefix = "annotated."
		}

		injector, _ := dingo.NewInjector()
		injector.Bind(new(Target)).AnnotatedWith(annotation).ToInstance(&Target{"orig"})
		injector.Override(new(*Target), annotation).ToInstance(&Target{"over"})
		r.add(prefix+"init", catchErr(func() error { return injector.InitModules() }))
		readTarget(r, prefix, injector, annotation)
	}
}

// probeOverridePointerOnPointerBinding binds *Target and overrides *Target.
func probeOverridePointerOnPointerBinding(r *recorder) {
	injector, _ := dingo.NewInjector()
	injector.Bind(new(*Target)).ToInstance(&Target{"orig"})
	injector.Override(new(*Target), "").ToInstance(&Target{"over"})

	r.add("init", catchErr(func() error { return injector.InitModules() }))
	readTarget(r, "", injector, "")
}

// probeOverridePointerWithValueAndPointerBound binds Target and *Target, then
// overrides *Target.
func probeOverridePointerWithValueAndPointerBound(r *recorder) {
	injector, _ := dingo.NewInjector()
	injector.Bind(new(Target)).ToInstance(&Target{"value"})
	injector.Bind(new(*Target)).ToInstance(&Target{"pointer"})
	injector.Override(new(*Target), "").ToInstance(&Target{"over"})

	r.add("init", catchErr(func() error { return injector.InitModules() }))
	readTarget(r, "", injector, "")
}

// probeOverridePointerInChild overrides *Target in a child area whose parent
// binds Target, without and with a Target binding in the child.
func probeOverridePointerInChild(r *recorder) {
	root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Target)).ToInstance(&Target{"root"})
	}))

	child, err := newArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Override(new(*Target), "").ToInstance(&Target{"over"})
	}))
	r.add("without-child-binding.init", errText(err))
	readTarget(r, "without-child-binding.", child, "")

	child, err = newArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Target)).ToInstance(&Target{"child"})
		injector.Override(new(*Target), "").ToInstance(&Target{"over"})
	}))
	r.add("with-child-binding.init", errText(err))
	readTarget(r, "with-child-binding.", child, "")
	readTarget(r, "root.", root, "")
}

// probeBindPointerTypeOnly binds *Target and reads Target every way.
func probeBindPointerTypeOnly(r *recorder) {
	injector, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(*Target)).ToInstance(&Target{"bound"})
	}))

	r.add("init", errText(err))
	readTarget(r, "", injector, "")
	r.add("GetInstance(*Target)", catch(func() string { return result(injector.GetInstance(new(*Target))) }))
	r.add("GetInstance(reflect:*Target)", catch(func() string { return result(injector.GetInstance(pointerToTarget)) }))
	r.add("field(**Target)", catch(func() string {
		var h struct {
			T **Target `inject:""`
		}

		err := injector.RequestInjection(&h)

		return errText(err) + fmt.Sprintf(" set=%v", h.T != nil)
	}))
}
