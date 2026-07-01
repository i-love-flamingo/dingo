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
