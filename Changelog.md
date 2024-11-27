# Changelog

## Version v0.3.0 (2024-11-27)

### Features

- **dingo:** introduce injection tracing (#68) (21c94ffa)

### Chores and tidying

- **deps:** update module github.com/stretchr/testify to v1.10.0 (#69) (ef77a754)
- update to go 1.22 and adjust linter rules (#67) (5f499b2e)
- **deps:** update actions/checkout action to v4 (#60) (8e15da8c)
- **deps:** update actions/setup-go action to v5 (#61) (d87ff516)
- **deps:** update golangci/golangci-lint-action action to v6 (#66) (bc363bf9)
- **deps:** update golangci/golangci-lint-action action to v5 (#65) (99893da6)
- **deps:** update module github.com/stretchr/testify to v1.9.0 (#63) (a327329b)
- update to go 1.21 minimum (#64) (e7b3183d)
- **deps:** update module github.com/stretchr/testify to v1.8.4 (#59) (8b5d6f5e)
- add regex to detect go run/install commands (88d11623)
- **deps:** update module github.com/stretchr/testify to v1.8.2 (#55) (34f5abd5)
- **deps:** update actions/setup-go action to v4 (#56) (c617aa9b)

## Version v0.2.10 (2022-11-04)

### Fixes

- **deps:** update module github.com/stretchr/testify to v1.7.1 (75d21325)

### Ops and CI/CD

- adjust gloangci-lint config for github actions (72b6052c)
- make "new-from-rev" work for golangci-lint (76e21ab6)
- remove unnecessary static checks (now handled by golangci-lint) (26838281)
- introduce golangci-lint (213edd33)
- **github:** update CI pipeline (f8a46198)
- **semanticore:** change default branch (263538dc)
- **semanticore:** add semanticore (10133afa)
- switch to GitHub Actions, bump go versions and deps (#24) (6df0d341)

### Chores and tidying

- **deps:** update irongut/codecoveragesummary action to v1.3.0 (#46) (7aae2361)
- **deps:** update module github.com/stretchr/testify to v1.8.1 (#49) (00c4e9e1)
- bump to go1.18 (a3c98a77)
- bump to go1.18 (aae76969)
- **deps:** update module github.com/stretchr/testify to v1.8.0 (#44) (b5fdfaa2)
- **renovate:** use chore commit message (c4ee6660)
- **deps:** update actions/setup-go action to v3 (47d41e94)
- **deps:** update actions/checkout action to v3 (4a2de875)
- **deps:** add renovate.json (6a6974a3)
- update readme (b6cd3f74)

### Other

- Add Testcase and handle pointer to interfaces in a guard clause (339d5584)
- [#7]: failing on usage of struct receiver in 'Inject' method (bea6ce02)
- remove unnecessary continue in dingo.go (a85f8336)
- require go 1.13 (76908a07)

