# Dingo — bugs surfaced by the test suite

> Tests-only effort: bugs are documented here, **not fixed**. Each entry links to a
> `t.Skip("BUG: … — see DINGO-FINDINGS.md#N")` test that reproduces it.

Skip-string convention:

    t.Skip("BUG: singleton scope returns cached value with nil error — see DINGO-FINDINGS.md#1")

## Findings

_(none confirmed yet — appended by later tasks as their tests go red)_

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
