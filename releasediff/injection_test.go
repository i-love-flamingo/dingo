package releasediff_test

import (
	"errors"
	"fmt"

	"flamingo.me/dingo"
)

// Probes on injection itself: providers, struct fields, nil values and the
// error texts callers see.

func init() {
	register("inject_providers", probeInjectProviders)
	register("inject_fields", probeInjectFields)
	register("inject_nil_values", probeInjectNilValues)
	register("inject_errors", probeInjectErrors)
}

type (
	TargetProvider     func() *Target
	TargetErrProvider  func() (*Target, error)
	ServiceProvider    func() Service
	ServiceErrProvider func() (Service, error)
	providerHolder     struct {
		Target     TargetProvider     `inject:""`
		TargetErr  TargetErrProvider  `inject:""`
		Unbound    ServiceProvider    `inject:""`
		UnboundErr ServiceErrProvider `inject:""`
		Annotated  HandlerProvider    `inject:"named"`
		Multi      []HandlerProvider  `inject:""`
	}
)

// command stands in for a cobra command bound by annotation, as Flamingo does
// with "flamingo".
type command struct{ use string }

// probeInjectProviders covers generated providers (func types named
// ...Provider), ToProvider bindings, and provider errors.
func probeInjectProviders(r *recorder) {
	injector := mustArea(nil, map[string]any{"site": "root"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).AnnotatedWith("named").ToProvider(func(s *siteHandler) Handler { return &namedHandler{"named:" + s.Site} })
		injector.BindMulti(new(Handler)).ToInstance(&namedHandler{"m1"})
		injector.BindMulti(new(Handler)).To(siteHandler{})
		injector.Bind(new(command)).AnnotatedWith("flamingo").ToProvider(func(h Handler) *command { return &command{use: "cmd:" + h.Name()} })
		injector.Bind(new(Handler)).To(siteHandler{})
	}))

	var h providerHolder

	r.add("inject", catchErr(func() error { return injector.RequestInjection(&h) }))
	r.add("target", catch(func() string { return fmt.Sprintf("%T", h.Target()) }))
	r.add("target-err", catch(func() string { v, err := h.TargetErr(); return errText(err) + fmt.Sprintf(" %T", v) }))
	r.add("unbound", catch(func() string { return show(h.Unbound()) }))
	r.add("unbound-err", catch(func() string { return result(h.UnboundErr()) }))
	r.add("annotated", catch(func() string { return show(h.Annotated()) }))
	r.add("multi", catch(func() string {
		handlers := make([]Handler, 0, len(h.Multi))
		for _, p := range h.Multi {
			handlers = append(handlers, p())
		}

		return handlerNames(handlers, nil)
	}))
	r.add("annotated-command", catch(func() string {
		v, err := injector.GetAnnotatedInstance(new(command), "flamingo")
		if err != nil {
			return result(nil, err)
		}

		return v.(*command).use
	}))
	r.add("provider-in-child", catch(func() string {
		fr := mustArea(injector, map[string]any{"site": "fr"})

		var h2 providerHolder
		if err := fr.RequestInjection(&h2); err != nil {
			return errText(err)
		}

		return show(h2.Annotated())
	}))
	r.add("provider-unbound-argument", catch(func() string {
		i, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).ToProvider(func(Service) Handler { return &namedHandler{"x"} })
		}))
		if err != nil {
			return errText(err)
		}

		return result(i.GetInstance(new(Handler)))
	}))
	r.add("provider-returns-nil", catch(func() string {
		i, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Handler)).ToProvider(func() Handler { return nil })
		}))

		return result(i.GetInstance(new(Handler)))
	}))
	r.add("error-provider-wraps", catch(func() string {
		var h3 struct {
			P HandlerErrProvider `inject:"nope"`
		}
		if err := injector.RequestInjection(&h3); err != nil {
			return errText(err)
		}

		v, err := h3.P()

		return result(v, err) + fmt.Sprintf(" wraps=%v", errors.Unwrap(err) != nil)
	}))
}

type (
	privateFieldHolder struct {
		leaf *Leaf `inject:""` //nolint:unused // dingo reaches it through reflection
	}
	embedded       struct{ X string }
	embeddedHolder struct {
		embedded `inject:""`
	}
	embeddedPointerHolder struct {
		*Leaf `inject:""`
	}
	interfacePointerHolder struct {
		H *Handler `inject:""`
	}
	valueReceiverInject struct{}
	setupHolder         struct {
		got      string
		leaf     *Leaf `inject:""` //nolint:unused // dingo reaches it through reflection
		Exported *Leaf `inject:""`
	}
	injectorHolder struct {
		I *dingo.Injector `inject:""`
	}
)

func (valueReceiverInject) Inject() {}

func (s *setupHolder) Inject(l *Leaf, h Handler) {
	s.got = fmt.Sprintf("leaf=%v handler=%s", l != nil, h.Name())
}

// probeInjectFields covers RequestInjection on unusual targets: unexported,
// embedded and pointer-to-interface fields, Inject methods, non-pointers,
// slices, injection requested during module setup, and the Injector itself.
func probeInjectFields(r *recorder) {
	injector := mustArea(nil, map[string]any{"site": "root"}, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).ToInstance(&namedHandler{"h"})
	}))

	r.add("unexported-field", catchErr(func() error { var p privateFieldHolder; return injector.RequestInjection(&p) }))
	r.add("embedded-struct", catchErr(func() error { var p embeddedHolder; return injector.RequestInjection(&p) }))
	r.add("embedded-pointer", catch(func() string {
		var p embeddedPointerHolder

		err := injector.RequestInjection(&p)

		return errText(err) + fmt.Sprintf(" set=%v", p.Leaf != nil)
	}))
	r.add("pointer-to-interface", catchErr(func() error { var p interfacePointerHolder; return injector.RequestInjection(&p) }))
	r.add("value-receiver-inject", catchErr(func() error { return injector.RequestInjection(valueReceiverInject{}) }))
	r.add("value-receiver-inject-is", catch(func() string {
		return fmt.Sprint(errors.Is(injector.RequestInjection(valueReceiverInject{}), dingo.ErrInvalidInjectReceiver))
	}))
	r.add("inject-method-then-unexported", catch(func() string {
		var s setupHolder

		err := injector.RequestInjection(&s)

		return errText(err) + " " + s.got
	}))
	r.add("non-pointer-struct", catchErr(func() error { return injector.RequestInjection(configHolder{}) }))
	r.add("nil", catchErr(func() error { return injector.RequestInjection(nil) }))
	r.add("nil-pointer", catchErr(func() error { return injector.RequestInjection((*Mid)(nil)) }))
	r.add("slice-of-pointers", catch(func() string {
		s := []*Mid{{}, {}}
		err := injector.RequestInjection(s)

		return errText(err) + fmt.Sprintf(" set=%v,%v", s[0].L != nil, s[1].L != nil)
	}))
	r.add("delayed-during-configure", catch(func() string {
		var m Mid

		_, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			if err := injector.RequestInjection(&m); err != nil {
				panic(err)
			}

			injector.Bind(new(Leaf)).ToInstance(&Leaf{X: "bound-later"})
		}))
		if err != nil {
			return errText(err)
		}

		return m.L.X
	}))
	r.add("injector-field", catch(func() string {
		child := mustArea(injector, nil)

		var fromChild, fromRoot injectorHolder

		_ = child.RequestInjection(&fromChild)
		_ = injector.RequestInjection(&fromRoot)

		return fmt.Sprintf("child-gets-child=%v root-gets-root=%v", fromChild.I == child, fromRoot.I == injector)
	}))
}

// probeInjectNilValues binds typed nil pointers and asks for nil.
func probeInjectNilValues(r *recorder) {
	injector := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(Handler)).ToInstance((*namedHandler)(nil))
		injector.BindMap(new(Handler), "nilptr").ToInstance((*namedHandler)(nil))
	}))
	fr := mustArea(injector, nil)

	r.add("GetInstance(nil)", catch(func() string { return result(injector.GetInstance(nil)) }))
	r.add("typed-nil-instance", catch(func() string {
		v, err := injector.GetInstance(new(Handler))

		return errText(err) + fmt.Sprintf(" %T nil=%v", v, v.(*namedHandler) == nil)
	}))
	r.add("typed-nil-map-key-in-child", catch(func() string {
		v, err := fr.GetAnnotatedInstance(new(Handler), "map:nilptr")
		if err != nil {
			return errText(err)
		}

		return fmt.Sprintf("%T nil=%v", v, v.(*namedHandler) == nil)
	}))
	r.add("child-GetInstance(Injector)", catch(func() string {
		v, err := fr.GetInstance(new(dingo.Injector))
		if err != nil {
			return errText(err)
		}

		return fmt.Sprintf("is-child=%v", v == fr)
	}))
}

type (
	cycleModuleA struct{}
	cycleModuleB struct{}
	selfModule   struct{}
)

func (*cycleModuleA) Configure(*dingo.Injector) {}
func (*cycleModuleA) Depends() []dingo.Module   { return []dingo.Module{new(cycleModuleB)} }
func (*cycleModuleB) Configure(*dingo.Injector) {}
func (*cycleModuleB) Depends() []dingo.Module   { return []dingo.Module{new(cycleModuleA)} }
func (*selfModule) Configure(*dingo.Injector)   { record("self") }
func (*selfModule) Depends() []dingo.Module     { return []dingo.Module{new(selfModule)} }

type customSingleton struct{ dingo.SingletonScope }

// probeInjectErrors records the error and panic texts of common mistakes, and
// which sentinel errors they match.
func probeInjectErrors(r *recorder) {
	injector, _ := dingo.NewInjector()

	r.add("unbound-interface", catch(func() string { return result(injector.GetInstance(new(Service))) }))
	r.add("annotated-missing", catch(func() string { return result(injector.GetAnnotatedInstance(new(Leaf), "x")) }))
	r.add("func-not-provider", catch(func() string { return result(injector.GetInstance(new(func() int))) }))
	r.add("field-unbound", catch(func() string {
		var h struct {
			S Service `inject:""`
		}

		return errText(injector.RequestInjection(&h))
	}))
	r.add("optional-unbound", catch(func() string {
		var h struct {
			S Service `inject:",optional"`
			L *Leaf   `inject:"nope,optional"`
		}

		err := injector.RequestInjection(&h)

		return errText(err) + fmt.Sprintf(" S=%v L=%+v", h.S, h.L)
	}))
	r.add("duplicate", catch(func() string {
		_, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Service)).To(realService{})
			injector.Bind(new(Service)).To(fakeService{})
		}))

		return errText(err)
	}))
	r.add("duplicate-equal", catch(func() string {
		_, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(Service)).To(realService{})
			injector.Bind(new(Service)).To(realService{})
		}))

		return errText(err)
	}))
	r.add("child-rebinds-parent", catch(func() string {
		root := mustArea(nil, nil, dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(Service)).To(realService{}) }))
		child := mustArea(root, nil, dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(Service)).To(fakeService{}) }))

		return result(child.GetInstance(new(Service)))
	}))
	r.add("unknown-scope", catch(func() string {
		i, _ := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) { injector.Bind(new(Leaf)).In(&customSingleton{}) }))

		return result(i.GetInstance(new(Leaf)))
	}))
	r.add("bind-nil", catch(func() string { injector.Bind(nil); return "ok" }))
	r.add("to-not-assignable", catch(func() string { injector.Bind(new(Service)).To(Leaf{}); return "ok" }))
	r.add("toinstance-nil", catch(func() string { injector.Bind(new(Service)).ToInstance(nil); return "ok" }))
	r.add("toprovider-not-assignable", catch(func() string {
		injector.Bind(new(Service)).ToProvider(func() *Leaf { return nil })

		return "ok"
	}))
	r.add("child-of-nil", catch(func() string { var n *dingo.Injector; _, err := n.Child(); return errText(err) }))
	r.add("module-cycle", catch(func() string {
		_, err := dingo.NewInjector(dingo.ModuleFunc(func(*dingo.Injector) {}), new(cycleModuleA))

		return fmt.Sprintf("is-ErrInitModules=%v is-ErrModuleCycle=%v", errors.Is(err, dingo.ErrInitModules), errors.Is(err, dingo.ErrModuleCycle))
	}))
	r.add("module-self-dependency", catch(func() string {
		_, err := dingo.NewInjector(new(selfModule))

		return fmt.Sprintf("is-ErrInitModules=%v is-ErrModuleCycle=%v", errors.Is(err, dingo.ErrInitModules), errors.Is(err, dingo.ErrModuleCycle))
	}))
	r.add("sentinels", fmt.Sprintf("%q %q %q %q",
		dingo.ErrInitModules, dingo.ErrInvalidInjectReceiver, dingo.ErrModuleCycle, dingo.ErrModuleSort))
	r.add("stages", fmt.Sprintf("INIT=%d DEFAULT=%d", dingo.INIT, dingo.DEFAULT))
}
