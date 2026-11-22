# Contributing to dingo

Open an issue or a pull request on GitHub. Commit messages follow
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/), for example `fix(dingo): ...`.

## Contract tests

The tests in `contract/` pin the behaviour that applications use. Each test checks one rule, and its name
tells the rule. A `TestLatent_` test pins a surprising behaviour that applications depend on. A change to it
is a breaking change.

- A pull request that changes, deletes or renames a file in `contract/` or `.github/workflows/` needs the
  `contract-change` label. A maintainer adds it after the review. Write a behaviour change into the release
  notes. The check covers changed, deleted and renamed files only, so a reviewer must read new files under
  `contract/` and `.github/workflows/`, for example a new `TestMain` or `init`.
- If an application reports a break after a dingo update, and the cause is not in the application code, the
  pull request with the correction also adds a contract test for that break.
- The `TestPerf_` tests limit the allocations of the resolve paths. Run them with
  `go test -run '^TestPerf_' ./contract/...`, without `-race`.
- `Downstream (public)` runs the tests of flamingo and flamingo-commerce against the pull request. It
  blocks only when their tests pass with the base commit. With the `contract-change` label, a downstream
  break is a warning only.
