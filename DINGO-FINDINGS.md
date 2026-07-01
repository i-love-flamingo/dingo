# Dingo — bugs surfaced by the test suite

> Tests-only effort: bugs are documented here, **not fixed**. Each entry links to a
> `t.Skip("BUG: … — see DINGO-FINDINGS.md#N")` test that reproduces it.

Skip-string convention:

    t.Skip("BUG: singleton scope returns cached value with nil error — see DINGO-FINDINGS.md#1")

## Findings

### 1. Self-dependent module panics instead of returning ErrModuleCycle
- **What's wrong:** A module whose `Depends()` returns itself does not surface as
  `ErrModuleCycle`. Instead, `modGraph.addModule` registers the module's graph node
  into `mg.index` *before* walking its dependencies, so when it depends on itself the
  recursive `addModule` call finds the same (already-registered) node ID and tries to
  add a self-loop edge. `gonum`'s `simple.DirectedGraph.SetEdge` panics on self-edges
  (`"simple: adding self edge"`), which is unrecovered and crashes the caller —
  `NewInjector` does not catch it.
- **Location:** `module.go:172-201` (`modGraph.addModule`), specifically the
  `mg.SetEdge(isDependencyOf)` call at `module.go:196`; the panic originates in
  `vendor/gonum.org/v1/gonum/graph/simple/directed.go:200`.
- **Reproducing test:** `TestSelfDependencyIsRejected` in `module_loading_test.go`
  (currently `t.Skip`, placed before the panicking call).
- **Observed:** `NewInjector` panics with `simple: adding self edge` instead of
  returning an error. **Expected:** `NewInjector` returns `ErrModuleCycle` (or a
  wrapped variant naming the module), matching the behavior for multi-module cycles
  (`TestDirectCycleIsRejected`, `TestCycleAmongSomeModulesStillRejected`).
  **Severity:** medium (self-dependency is presumably a rare authoring mistake, but a
  panic — as opposed to a returned error — can crash a calling process instead of
  letting it handle initialization failure gracefully).

### 2. `ToProvider` binding silently drops the provider's error return
- **What's wrong:** When a binding is created with `Bind(...).ToProvider(fn)` where
  `fn` has signature `func(...) (T, error)`, and that binding is resolved through the
  normal `getInstance`/field-injection path (as opposed to a multibinding/mapbinding
  `...Provider`-suffixed slice/map field), the error return of `fn` is never inspected.
  `Provider.Create` calls `p.fnc.Call(in)[0]` — hardcoding index `0` — and returns
  `injector.requestInjection(res, traceCircular)` as the error, completely discarding
  whatever `fn` returned as its second value. A provider that deliberately returns
  `(nil, someErr)` therefore resolves successfully with a nil/zero value instead of
  surfacing `someErr`.
- **Location:** `binding.go:103-116` (`Provider.Create`), specifically
  `res := p.fnc.Call(in)[0]` at `binding.go:114`, which only reads the first return
  value. Contrast with `dingo.go:470-502` (`createProviderForBinding`), which
  correctly threads a `canError` flag and reflects the second return value out via
  `reflectedError` — but that code path is only used for multibindings/mapbindings of
  providers, not for a plain `ToProvider` binding consumed via `GetInstance` or an
  injected `...Provider`-suffixed field backed by that binding.
- **Reproducing test:** `TestProviderErrorPropagates` in `injector_resolution_test.go`
  (currently `t.Skip`).
- **Observed:** `got.(*holder).P()` returns `(nil, nil)` — the error is swallowed.
  **Expected:** `got.(*holder).P()` returns `(nil, boom)`, surfacing the provider's
  error to the caller. **Severity:** medium-high (silently converts a provider failure
  into a successful-looking nil/zero value, which can propagate corrupt state further
  into the application undetected).

## Intentionally not covered (a choice, not an oversight)

- logging toggles `EnableInjectionTracing` / `EnableCircularTracing`
- the `errUnbound.Error` string formatter

<!--
Template:

### N. <title>
- **What's wrong:** <plain description>
- **Location:** `scope.go:56-64`
- **Reproducing test:** `TestName` in `<file>` (currently t.Skip)
- **Observed:** … **Expected:** … **Severity:** high|medium|low
-->
