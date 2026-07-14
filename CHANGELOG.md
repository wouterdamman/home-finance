# Changelog

## [1.2.0](https://github.com/wouterdamman/home-finance/compare/v1.1.0...v1.2.0) (2026-07-14)


### Features

* harden period-close correctness, add rate limiting and infra hardening ([f6f46d4](https://github.com/wouterdamman/home-finance/commit/f6f46d426cb4bc9df73d6961a92cabaa59f0eeb6))


### Bug Fixes

* CI never ran migrations before integration tests, and a bogus trivy-action tag ([6541789](https://github.com/wouterdamman/home-finance/commit/65417894d00ff4d2f02f2368cccec2f4dc00ee6c))
* **deps:** update dependency react-router-dom to v7 ([#35](https://github.com/wouterdamman/home-finance/issues/35)) ([3a468ae](https://github.com/wouterdamman/home-finance/commit/3a468ae1d445e48b92cf3aa62028c6135fd44247))
* **deps:** update mantine monorepo to v9 ([#37](https://github.com/wouterdamman/home-finance/issues/37)) ([37cc6e6](https://github.com/wouterdamman/home-finance/commit/37cc6e640a18749ff6e8fb1b7f067d868ea299b2))
* **deps:** update module github.com/caarlos0/env/v11 to v11.4.1 ([#20](https://github.com/wouterdamman/home-finance/issues/20)) ([58d0e5b](https://github.com/wouterdamman/home-finance/commit/58d0e5bf590f645d85aafd01607eed3aee766217))
* **deps:** update module github.com/coreos/go-oidc/v3 to v3.20.0 ([#21](https://github.com/wouterdamman/home-finance/issues/21)) ([38b454c](https://github.com/wouterdamman/home-finance/commit/38b454cb454b890104eff2e2b86875ded25ee861))
* **deps:** update module github.com/go-chi/chi/v5 to v5.3.1 ([#16](https://github.com/wouterdamman/home-finance/issues/16)) ([a7b775b](https://github.com/wouterdamman/home-finance/commit/a7b775b7cb710bf708ac01c84ad4e9de46f0dbbd))
* **deps:** update module github.com/jackc/pgx/v5 to v5.10.0 ([#22](https://github.com/wouterdamman/home-finance/issues/22)) ([b3c68eb](https://github.com/wouterdamman/home-finance/commit/b3c68eb6318e31493600aca645d21ef8bdef7cc8))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.2 ([#23](https://github.com/wouterdamman/home-finance/issues/23)) ([1d94190](https://github.com/wouterdamman/home-finance/commit/1d94190254f2c24f68f8fbe52be4f4a92e2c9c23))
* resolve Dex OIDC issuer mismatch between host and containerized api ([0b63c00](https://github.com/wouterdamman/home-finance/commit/0b63c005fcd55dcdb8f0ba54489cc946cc1bde65))

## [1.1.0](https://github.com/wouterdamman/home-finance/compare/v1.0.1...v1.1.0) (2026-07-14)


### Features

* remove Spaarpotjes nav and pages ([29453d9](https://github.com/wouterdamman/home-finance/commit/29453d910024dd6710690b9105172570cc911768))
* year locking — lock/unlock a year with password ([8d9e4e5](https://github.com/wouterdamman/home-finance/commit/8d9e4e58dc5e91594f05f612371037b4a490677d))


### Bug Fixes

* **deps:** update module github.com/xuri/excelize/v2 to v2.11.0 [security] ([#8](https://github.com/wouterdamman/home-finance/issues/8)) ([f69b718](https://github.com/wouterdamman/home-finance/commit/f69b7182cdb25471b8df3ee2746ae93671c764e5))

## [1.0.1](https://github.com/wouterdamman/home-finance/compare/v1.0.0...v1.0.1) (2026-07-05)


### Bug Fixes

* apply security and code-quality fixes from v1.0.0 review ([e5bede7](https://github.com/wouterdamman/home-finance/commit/e5bede7633430a39779e4db0e5f1c8951ab536a2))
