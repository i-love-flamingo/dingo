package dingo

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

const (
	// INIT state
	INIT = iota
	// DEFAULT state
	DEFAULT
)

var (
	ErrInvalidInjectReceiver = errors.New("usage of 'Inject' method with struct receiver is not allowed")
	errPointersToInterface   = errors.New(" Do not use pointers to interface.")

	traceCircular    []circularTraceEntry
	injectionTracing = false
)

// EnableCircularTracing activates dingo's trace feature to find circular dependencies
// this is super expensive (memory wise), so it should only be used for debugging purposes
func EnableCircularTracing() {
	traceCircular = make([]circularTraceEntry, 0)
	_ = slog.SetLogLoggerLevel(slog.LevelDebug)
}

func EnableInjectionTracing() {
	injectionTracing = true
	_ = slog.SetLogLoggerLevel(slog.LevelDebug)
}

type (
	// Injector defines bindings and multibindings
	// it is possible to have a parent-injector, which can be asked if no resolution is available
	Injector struct {
		bindings             map[reflect.Type][]*Binding          // list of available bindings for a concrete type
		multibindings        map[reflect.Type][]*Binding          // list of multi-bindings for a concrete type
		mapbindings          map[reflect.Type]map[string]*Binding // list of map-bindings for a concrete type
		interceptor          map[reflect.Type][]reflect.Type      // list of interceptors for a type
		overrides            []*override                          // list of overrides for a binding
		parent               *Injector                            // parent injector reference
		scopes               map[reflect.Type]Scope               // scope-bindings
		stage                uint                                 // current stage
		delayed              []interface{}                        // delayed bindings
		buildEagerSingletons bool                                 // weather to build singletons
	}

	// overrides are evaluated lazy, so they are scheduled here
	override struct {
		typ           reflect.Type
		annotatedWith string
		binding       *Binding
	}

	circularTraceEntry struct {
		typ        reflect.Type
		annotation string
	}
)

// NewInjector builds up a new Injector out of a list of Modules
func NewInjector(modules ...Module) (*Injector, error) {
	injector := &Injector{
		bindings:             make(map[reflect.Type][]*Binding),
		multibindings:        make(map[reflect.Type][]*Binding),
		mapbindings:          make(map[reflect.Type]map[string]*Binding),
		interceptor:          make(map[reflect.Type][]reflect.Type),
		scopes:               make(map[reflect.Type]Scope),
		stage:                DEFAULT,
		buildEagerSingletons: true,
	}

	// bind current injector
	injector.Bind(Injector{}).ToInstance(injector)

	// bind default scopes
	injector.BindScope(Singleton)
	injector.BindScope(ChildSingleton)

	// init current modules
	return injector, injector.InitModules(modules...)
}

// Child derives a child injector with a new ChildSingletonScope
func (injector *Injector) Child() (*Injector, error) {
	if injector == nil {
		return nil, errors.New("can not create a child of an uninitialized injector")
	}

	newInjector, err := NewInjector()
	if err != nil {
		return nil, err
	}

	newInjector.parent = injector
	newInjector.Bind(Injector{}).ToInstance(newInjector)
	newInjector.BindScope(NewChildSingletonScope()) // bind a new child-singleton

	return newInjector, nil
}

// InitModules initializes the injector with the given modules
func (injector *Injector) InitModules(modules ...Module) error {
	injector.stage = INIT

	modules = resolveDependencies(modules, nil)
	for _, module := range modules {
		if err := injector.requestInjection(module, traceCircular); err != nil {
			erroredModule := reflect.TypeOf(module).Elem()
			return fmt.Errorf("initmodules: injection into %q failed: %w", erroredModule.PkgPath()+"."+erroredModule.Name(), err)
		}
		module.Configure(injector)
	}

	// evaluate overrides when modules were loaded
	for _, override := range injector.overrides {
		bindtype := override.typ
		if bindtype.Kind() == reflect.Ptr {
			bindtype = bindtype.Elem()
		}
		if bindings, ok := injector.bindings[bindtype]; ok && len(bindings) > 0 {
			for i, binding := range bindings {
				if binding.annotatedWith == override.annotatedWith {
					injector.bindings[bindtype][i] = override.binding
				}
			}
			continue
		}
		return fmt.Errorf("cannot override unknown binding %q (annotated with %q)", override.typ.String(), override.annotatedWith) // todo ok?
	}

	// make sure there are no duplicated bindings
	for typ, bindings := range injector.bindings {
		known := make(map[string]*Binding)
		for _, binding := range bindings {
			if known, ok := known[binding.annotatedWith]; ok && !known.equal(binding) {
				var knownBinding, duplicateBinding string
				if known.to != nil {
					knownBinding = fmt.Sprintf("%#v%#v", known.to.PkgPath(), known.to.Name())
				}
				if binding.to != nil {
					duplicateBinding = fmt.Sprintf("%#v%#v", binding.to.PkgPath(), binding.to.Name())
				}
				return fmt.Errorf("already known binding for %q with annotation %q | Known binding: %q Try %q", typ, binding.annotatedWith, knownBinding, duplicateBinding)
			}
			known[binding.annotatedWith] = binding
		}
	}

	injector.stage = DEFAULT

	// continue with delayed injections
	for _, object := range injector.delayed {
		if err := injector.requestInjection(object, traceCircular); err != nil {
			return err
		}
	}

	injector.delayed = nil

	// build eager singletons
	if !injector.buildEagerSingletons {
		return nil
	}
	return injector.BuildEagerSingletons(false)
}

// SetBuildEagerSingletons can be used to disable or enable building of eager singletons during InitModules
func (injector *Injector) SetBuildEagerSingletons(build bool) {
	injector.buildEagerSingletons = build
}

// BuildEagerSingletons requests one instance of each singleton, optional letting the parent injector(s) do the same
func (injector *Injector) BuildEagerSingletons(includeParent bool) error {
	for _, bindings := range injector.bindings {
		for _, binding := range bindings {
			if binding.eager {
				if _, err := injector.getInstance(binding.typeof, binding.annotatedWith, traceCircular); err != nil {
					return fmt.Errorf("initmodules: loading eager singletons: %w", err)
				}
			}
		}
	}
	if includeParent && injector.parent != nil {
		return injector.parent.BuildEagerSingletons(includeParent)
	}
	return nil
}

// GetInstance creates a new instance of what was requested
func (injector *Injector) GetInstance(of interface{}) (interface{}, error) {
	i, err := injector.getInstance(of, "", traceCircular)
	if err != nil {
		return nil, err
	}
	return i.Interface(), nil
}

// GetAnnotatedInstance creates a new instance of what was requested with the given annotation
func (injector *Injector) GetAnnotatedInstance(of interface{}, annotatedWith string) (interface{}, error) {
	i, err := injector.getInstance(of, annotatedWith, traceCircular)
	if err != nil {
		return nil, err
	}
	return i.Interface(), nil
}

// getInstance creates the new instance of typ, returns a reflect.value
func (injector *Injector) getInstance(typ interface{}, annotatedWith string, circularTrace []circularTraceEntry) (reflect.Value, error) {
	oftype := reflect.TypeOf(typ)

	if oft, ok := typ.(reflect.Type); ok {
		oftype = oft
	} else {
		for oftype.Kind() == reflect.Ptr {
			oftype = oftype.Elem()
		}
	}

	return injector.getInstanceOfTypeWithAnnotation(oftype, annotatedWith, nil, false, circularTrace)
}

func (injector *Injector) findBindingForAnnotatedType(t reflect.Type, annotation string) *Binding {
	if len(injector.bindings[t]) > 0 {
		for _, binding := range injector.bindings[t] {
			if binding.annotatedWith == annotation {
				return binding
			}
		}
	}

	// inject one key of a map-binding
	if len(annotation) > 4 && annotation[:4] == "map:" {
		return injector.mapbindings[t][annotation[4:]]
	}

	// ask parent
	if injector.parent != nil {
		return injector.parent.findBindingForAnnotatedType(t, annotation)
	}

	return nil
}

// getInstanceOfTypeWithAnnotation resolves a requested type, with annotation
func (injector *Injector) getInstanceOfTypeWithAnnotation(t reflect.Type, annotation string, binding *Binding, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var final reflect.Value
	var err error

	if typeBinding := injector.findBindingForAnnotatedType(t, annotation); typeBinding != nil {
		binding = typeBinding
	}
	if binding != nil {
		if binding.scope != nil {
			if scope, ok := injector.scopes[reflect.TypeOf(binding.scope)]; ok {
				if final, err = scope.ResolveType(t, annotation, func(t reflect.Type, annotation string, optional bool) (reflect.Value, error) {
					return injector.createInstanceOfAnnotatedType(t, annotation, optional, circularTrace)
				}); err != nil {
					return reflect.Value{}, err
				}
				if !final.IsValid() {
					return reflect.Value{}, fmt.Errorf("%T did not resolve %s", scope, t)
				}
			} else {
				return reflect.Value{}, fmt.Errorf("unknown scope %T for %s", binding.scope, t)
			}
		}
	}

	if !final.IsValid() {
		if final, err = injector.createInstanceOfAnnotatedType(t, annotation, optional, circularTrace); err != nil {
			return reflect.Value{}, err
		}
	}

	if !final.IsValid() {
		return reflect.Value{}, fmt.Errorf("can not resolve %q", t.String())
	}

	return injector.intercept(final, t)
}

func (injector *Injector) intercept(final reflect.Value, t reflect.Type) (reflect.Value, error) {
	for _, interceptor := range injector.interceptor[t] {
		of := final
		final = reflect.New(interceptor)
		if err := injector.requestInjection(final.Interface(), traceCircular); err != nil {
			return reflect.Value{}, err
		}
		final.Elem().Field(0).Set(of)
	}
	if injector.parent != nil {
		return injector.parent.intercept(final, t)
	}
	return final, nil
}

type errUnbound struct {
	binding *Binding
	typ     reflect.Type
}

func (err errUnbound) Error() string {
	return fmt.Sprintf("binding is not bound: %v for %s", err.binding, err.typ)
}

func (injector *Injector) resolveBinding(binding *Binding, t reflect.Type, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	if binding.instance != nil {
		return binding.instance.ivalue, nil
	}

	if binding.provider != nil {
		return binding.provider.Create(injector)
	}

	if binding.to != nil {
		if binding.to == t {
			return reflect.Value{}, fmt.Errorf("circular from %q to %q (annotated with: %q)", t, binding.to, binding.annotatedWith)
		}
		return injector.getInstanceOfTypeWithAnnotation(binding.to, "", binding, optional, circularTrace)
	}

	return reflect.Value{}, errUnbound{binding: binding, typ: t}
}

// createInstanceOfAnnotatedType resolves a type request with the current injector
func (injector *Injector) createInstanceOfAnnotatedType(t reflect.Type, annotation string, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	if binding := injector.findBindingForAnnotatedType(t, annotation); binding != nil {
		r, err := injector.resolveBinding(binding, t, optional, circularTrace)
		if err == nil || !errors.As(err, new(errUnbound)) {
			return r, err
		}

		// todo: proper testcases
		if annotation != "" {
			return injector.getInstanceOfTypeWithAnnotation(binding.typeof, "", binding, false, circularTrace)
		}
	}

	// This for an injection request on a provider, such as `func() MyInstance`
	if t.Kind() == reflect.Func && (t.NumOut() == 1 || t.NumOut() == 2) && strings.HasSuffix(t.Name(), "Provider") {
		providerCanError := t.NumOut() == 2 && t.Out(1).AssignableTo(reflect.TypeOf(new(error)).Elem())
		if traceCircular != nil {
			return injector.createProvider(t, annotation, optional, providerCanError, make([]circularTraceEntry, 0)), nil
		}
		return injector.createProvider(t, annotation, optional, providerCanError, nil), nil
	}

	// This is the injection request for multibindings
	if t.Kind() == reflect.Slice {
		return injector.resolveMultibinding(t, annotation, optional, circularTrace)
	}

	// Map Binding injection
	if t.Kind() == reflect.Map && t.Key().Kind() == reflect.String {
		return injector.resolveMapbinding(t, annotation, optional, circularTrace)
	}

	if annotation != "" && !optional {
		return reflect.Value{}, fmt.Errorf("can not automatically create an annotated injection %q with annotation %q", t, annotation)
	}

	if t.Kind() == reflect.Interface && !optional {
		return reflect.Value{}, fmt.Errorf("can not instantiate interface %s.%s", t.PkgPath(), t.Name())
	}

	if t.Kind() == reflect.Func && !optional {
		return reflect.Value{}, fmt.Errorf("can not create a new function %q (Do you want a provider? Then suffix type with Provider)", t)
	}

	if circularTrace != nil {
		for _, ct := range circularTrace {
			if ct.typ == t && ct.annotation == annotation {
				for _, ct := range circularTrace {
					slog.Debug(fmt.Sprintf("%s#%s: %s", ct.typ.PkgPath(), ct.typ.Name(), ct.annotation))
				}

				slog.Debug(fmt.Sprintf("%s#%s: %s", t.PkgPath(), t.Name(), annotation))

				panic("detected circular dependency")
			}
		}
		subCircularTrace := make([]circularTraceEntry, len(circularTrace))
		copy(subCircularTrace, circularTrace)
		subCircularTrace = append(subCircularTrace, circularTraceEntry{t, annotation})

		n := reflect.New(t)
		return n, injector.requestInjection(n.Interface(), subCircularTrace)
	}

	if injectionTracing {
		if t.PkgPath() == "" || t.Name() == "" {
			slog.Debug(fmt.Sprintf("INJECTING: %s", t.String()))
		} else {
			slog.Debug(fmt.Sprintf("INJECTING: %s#%s \"%s\"", t.PkgPath(), t.Name(), annotation))
		}
	}

	n := reflect.New(t)
	return n, injector.requestInjection(n.Interface(), nil)
}

func reflectedError(err *error, t reflect.Type) reflect.Value {
	rerr := reflect.New(reflect.TypeOf(new(error)).Elem()).Elem()
	if err == nil || *err == nil {
		return rerr
	}
	rerr.Set(reflect.ValueOf(fmt.Errorf("%q: %w", t, *err)))
	return rerr
}

func (injector *Injector) createProvider(t reflect.Type, annotation string, optional bool, canError bool, circularTrace []circularTraceEntry) reflect.Value {
	return reflect.MakeFunc(t, func(args []reflect.Value) (results []reflect.Value) {
		// create a new type
		res := reflect.New(t.Out(0))
		// dereference possible interface pointer
		if res.Kind() == reflect.Ptr && (res.Elem().Kind() == reflect.Interface || res.Elem().Kind() == reflect.Ptr) {
			res = res.Elem()
		}

		ret := func(v reflect.Value, err error) []reflect.Value {
			if err != nil && !canError {
				panic(fmt.Errorf("%q: %w", t, err))
			} else if canError {
				return []reflect.Value{v, reflectedError(&err, t)}
			} else {
				return []reflect.Value{v}
			}
		}

		// multibindings
		if res.Elem().Kind() == reflect.Slice {
			return ret(injector.createInstanceOfAnnotatedType(t.Out(0), annotation, optional, circularTrace))
		}

		// mapbindings
		if res.Elem().Kind() == reflect.Map && res.Elem().Type().Key().Kind() == reflect.String {
			return ret(injector.createInstanceOfAnnotatedType(t.Out(0), annotation, optional, circularTrace))
		}

		r := ret(injector.getInstance(t.Out(0), annotation, circularTrace))

		res.Set(r[0])
		r[0] = res

		return r
	})
}

func (injector *Injector) createProviderForBinding(t reflect.Type, binding *Binding, annotation string, optional bool, canError bool, circularTrace []circularTraceEntry) reflect.Value {
	return reflect.MakeFunc(t, func(args []reflect.Value) (results []reflect.Value) {
		// create a new type
		res := reflect.New(binding.typeof)
		// dereference possible interface pointer
		if res.Kind() == reflect.Ptr && (res.Elem().Kind() == reflect.Interface || res.Elem().Kind() == reflect.Ptr) {
			res = res.Elem()
		}

		if r, err := injector.resolveBinding(binding, t, optional, circularTrace); err == nil {
			res.Set(r)
			if canError {
				return []reflect.Value{res, reflectedError(nil, t)}
			}
			return []reflect.Value{res}
		}

		// set to actual value
		i, err := injector.getInstance(binding.typeof, annotation, circularTrace)
		if err != nil {
			if canError {
				return []reflect.Value{res, reflectedError(&err, t)}
			}
			panic(fmt.Errorf("%q: %w", t, err))
		}
		res.Set(i)
		// return
		if canError {
			return []reflect.Value{res, reflectedError(nil, t)}
		}
		return []reflect.Value{res}
	})
}

func (injector *Injector) joinMultibindings(t reflect.Type, annotation string) []*Binding {
	var parent []*Binding
	if injector.parent != nil {
		parent = injector.parent.joinMultibindings(t, annotation)
	}

	bindings := make([]*Binding, len(parent)+len(injector.multibindings[t]))
	copy(bindings, parent)
	c := len(parent)
	for _, b := range injector.multibindings[t] {
		if b.annotatedWith == annotation {
			bindings[c] = b
			c++
		}
	}
	return bindings[:c]
}

func (injector *Injector) resolveMultibinding(t reflect.Type, annotation string, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	targetType := t.Elem()
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}

	providerType := targetType
	provider := strings.HasSuffix(targetType.Name(), "Provider") && targetType.Kind() == reflect.Func
	providerCanError := provider && targetType.NumOut() == 2 && targetType.Out(1).AssignableTo(reflect.TypeOf(new(error)).Elem())

	if provider {
		targetType = targetType.Out(0)
	}

	if bindings := injector.joinMultibindings(targetType, annotation); len(bindings) > 0 {
		n := reflect.MakeSlice(t, 0, len(bindings))
		for _, binding := range bindings {
			if provider {
				n = reflect.Append(n, injector.createProviderForBinding(providerType, binding, annotation, false, providerCanError, circularTrace))
				continue
			}

			r, err := injector.resolveBinding(binding, t, optional, circularTrace)
			if err != nil {
				return reflect.Value{}, err
			}
			n = reflect.Append(n, r)
		}
		return n, nil
	}

	return reflect.MakeSlice(t, 0, 0), nil
}

func (injector *Injector) joinMapbindings(t reflect.Type, annotation string) map[string]*Binding {
	var parent map[string]*Binding
	if injector.parent != nil {
		parent = injector.parent.joinMapbindings(t, annotation)
	}

	bindings := make(map[string]*Binding, len(parent)+len(injector.multibindings[t]))
	for k, v := range parent {
		bindings[k] = v
	}
	for k, v := range injector.mapbindings[t] {
		if v.annotatedWith == annotation {
			bindings[k] = v
		}
	}
	return bindings
}

func (injector *Injector) resolveMapbinding(t reflect.Type, annotation string, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	targetType := t.Elem()
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}

	providerType := targetType
	provider := strings.HasSuffix(targetType.Name(), "Provider") && targetType.Kind() == reflect.Func
	providerCanError := provider && targetType.NumOut() == 2 && targetType.Out(1).AssignableTo(reflect.TypeOf(new(error)).Elem())

	if provider {
		targetType = targetType.Out(0)
	}

	if bindings := injector.joinMapbindings(targetType, annotation); len(bindings) > 0 {
		n := reflect.MakeMapWithSize(t, len(bindings))
		for key, binding := range bindings {
			if provider {
				n.SetMapIndex(reflect.ValueOf(key), injector.createProviderForBinding(providerType, binding, annotation, false, providerCanError, circularTrace))
				continue
			}

			r, err := injector.resolveBinding(binding, t, optional, circularTrace)
			if err != nil {
				return reflect.Value{}, err
			}
			n.SetMapIndex(reflect.ValueOf(key), r)
		}
		return n, nil
	}

	return reflect.MakeMap(t), nil
}

// BindMulti binds multiple concrete types to the same abstract type / interface
func (injector *Injector) BindMulti(what interface{}) *Binding {
	bindtype := reflect.TypeOf(what)
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}
	binding := new(Binding)
	binding.typeof = bindtype
	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb
	return binding
}

// BindMap does a registry-like map-based binding, like BindMulti
func (injector *Injector) BindMap(what interface{}, key string) *Binding {
	bindtype := reflect.TypeOf(what)
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}
	binding := new(Binding)
	binding.typeof = bindtype
	bindingMap := injector.mapbindings[bindtype]
	if bindingMap == nil {
		bindingMap = make(map[string]*Binding)
	}
	bindingMap[key] = binding
	injector.mapbindings[bindtype] = bindingMap

	return binding
}

// BindInterceptor intercepts to interface with interceptor
func (injector *Injector) BindInterceptor(to, interceptor interface{}) {
	totype := reflect.TypeOf(to)
	if totype.Kind() == reflect.Ptr {
		totype = totype.Elem()
	}
	if totype.Kind() != reflect.Interface {
		panic("can only intercept interfaces " + fmt.Sprintf("%v", to))
	}
	m := injector.interceptor[totype]
	m = append(m, reflect.TypeOf(interceptor))
	injector.interceptor[totype] = m
}

// BindScope binds a scope to be aware of
func (injector *Injector) BindScope(s Scope) {
	injector.scopes[reflect.TypeOf(s)] = s
}

// Bind creates a new binding for an abstract type / interface
// Use the syntax
//
//	injector.Bind((*Interface)(nil))
//
// To specify the interface (cast it to a pointer to a nil of the type Interface)
func (injector *Injector) Bind(what interface{}) *Binding {
	if what == nil {
		panic("Cannot bind nil")
	}
	bindtype := reflect.TypeOf(what)
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}
	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)
	return binding
}

// Override a binding
func (injector *Injector) Override(what interface{}, annotatedWith string) *Binding {
	binding := injector.Bind(what).AnnotatedWith(annotatedWith)
	injector.overrides = append(injector.overrides, &override{typ: binding.typeof, annotatedWith: annotatedWith, binding: binding})
	return binding
}

// RequestInjection requests the object to have all fields annotated with `inject` to be filled
func (injector *Injector) RequestInjection(object interface{}) error {
	if injector.stage == INIT {
		injector.delayed = append(injector.delayed, object)
	} else {
		return injector.requestInjection(object, traceCircular)
	}
	return nil
}

func (injector *Injector) requestInjection(object interface{}, circularTrace []circularTraceEntry) error {
	if _, ok := object.(reflect.Value); !ok {
		object = reflect.ValueOf(object)
	}
	var injectlist = []reflect.Value{object.(reflect.Value)}
	var i int
	var current reflect.Value
	var err error

	wrapErr := func(err error) error {
		path := current.Type().PkgPath()
		if path == "" {
			if current.Kind() == reflect.Ptr {
				path = current.Elem().Type().PkgPath()
			}
		}
		if path != "" {
			path += "."
		}
		return fmt.Errorf("injecting into %s%s:\n%w", path, current.String(), err)
	}

	for {
		if i >= len(injectlist) {
			break
		}

		current = injectlist[i]
		ctype := current.Type()

		i++

		if ctype.Kind() != reflect.Ptr && current.MethodByName("Inject").IsValid() {
			return fmt.Errorf("invalid inject receiver %s: %w", current, ErrInvalidInjectReceiver)
		}

		switch ctype.Kind() {
		// dereference pointer
		case reflect.Ptr:
			if setup := current.MethodByName("Inject"); setup.IsValid() {
				args := make([]reflect.Value, setup.Type().NumIn())
				for i := range args {
					if args[i], err = injector.getInstance(setup.Type().In(i), "", circularTrace); err != nil {
						return wrapErr(err)
					}
				}
				setup.Call(args)
			}
			injectlist = append(injectlist, current.Elem())

		// inject into struct fields
		case reflect.Struct:
			for fieldIndex := 0; fieldIndex < ctype.NumField(); fieldIndex++ {
				if tag, ok := ctype.Field(fieldIndex).Tag.Lookup("inject"); ok {
					field := current.Field(fieldIndex)
					currentFieldName := ctype.Field(fieldIndex).Name
					if field.Kind() == reflect.Struct {
						return fmt.Errorf("can not inject into struct %#v of %#v", field, current)
					}

					var optional bool
					for _, option := range strings.Split(tag, ",") {
						switch strings.TrimSpace(option) {
						case "optional":
							optional = true
						}
					}
					tag = strings.Split(tag, ",")[0]

					instance, err := injector.getInstanceOfTypeWithAnnotation(field.Type(), tag, nil, optional, circularTrace)
					if err != nil {
						return wrapErr(err)
					}
					if instance.Kind() == reflect.Ptr {
						if instance.Elem().Kind() == reflect.Func || instance.Elem().Kind() == reflect.Interface || instance.Elem().Kind() == reflect.Slice {
							instance = instance.Elem()
						}
					}
					if field.Kind() != reflect.Ptr && field.Kind() != reflect.Interface && instance.Kind() == reflect.Ptr {
						if injectionTracing {
							slog.Debug(fmt.Sprintf("SETTING FIELD: %s of type \"%s\"", currentFieldName, ctype.Field(fieldIndex).Type.String()))
						}

						field.Set(instance.Elem())
					} else {
						if field.Kind() == reflect.Ptr && field.Type().Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Interface {
							return wrapErr(fmt.Errorf("field %#v is pointer to interface. %w", currentFieldName, errPointersToInterface))
						}

						if injectionTracing {
							slog.Debug(fmt.Sprintf("SETTING FIELD: %s of type \"%s\"", currentFieldName, ctype.Field(fieldIndex).Type.String()))
						}

						field.Set(instance)
					}
				}
			}

		case reflect.Interface:
			if !current.IsNil() {
				injectlist = append(injectlist, current.Elem())
			}

		case reflect.Slice:
			for i := 0; i < current.Len(); i++ {
				injectlist = append(injectlist, current.Index(i))
			}

		default:
		}
	}
	return nil
}
