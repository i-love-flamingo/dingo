# Release diff

This check compares how the dingo code in this repository behaves with how the
last release behaves, and fails unless every difference is listed in
`expected.txt`. It exists so that a behaviour change cannot slip into a release
unnoticed, even when the unit tests were changed along with it.

The probes in the `_test.go` files use dingo the way Flamingo does: area
injectors with config, map and multibindings, overrides, modules with
dependencies, tracing. Each probe runs in its own process, once as is and once
with circular tracing enabled, so a panic, stack overflow or hang only ends
that probe and becomes part of its result. A probe hangs when Go reports a
deadlock or when it runs longer than `RELEASEDIFF_TIMEOUT` (default 60s). Every
observation is one line:

```
<probe> <mode> <label>=<value>
```

`check.sh` runs the probes against the release required in `go.mod` and
against the code in this repository, and writes the lines that differ: `- `
for a line seen only with the release, `+ ` for a line seen only with the
current code. The result must match `expected.txt`.

## Running it

```sh
releasediff/check.sh
```

Set `RELEASEDIFF_RACE=1` to run the probes against the current code with the
race detector. The check then fails on any data race. A probe that hangs waits
for the timeout under the race detector, so this mode is slower. CI runs the
check on pull requests into master and on pushes to master.

## When a behaviour change is intended

Run `releasediff/check.sh -update` and commit the new `expected.txt`. The pull
request must explain each new or removed line, so that the reviewer agrees to
every change in behaviour, not only to the code.

If the check fails on a line you did not mean to change, the code changed
behaviour by accident. Fix the code, not `expected.txt`.

## Before a release

Every line in `expected.txt` is a behaviour change since the last release.
Before tagging, make sure each one is in the release notes. If a line should
not ship, fix the code instead.

## After a release

Move the requirement in `go.mod` to the new tag, run `go mod tidy` in this
directory and `releasediff/check.sh -update`. `expected.txt` should then be
empty: the release contains every change it listed.

To cover more behaviour, add a probe in the matching `_test.go` file and
register it in that file's `init`. Run the check before and after the change
you want to guard, so you see the probe report the difference.
