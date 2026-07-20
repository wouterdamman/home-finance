# Changelog

## [1.7.4](https://github.com/wouterdamman/home-finance/compare/v1.7.3...v1.7.4) (2026-07-20)


### Bug Fixes

* **pwa:** exclude /auth/ from service worker navigation fallback ([b8654a7](https://github.com/wouterdamman/home-finance/commit/b8654a76f8f71faad71c082471a6bab1d6d1eddf))

## [1.7.3](https://github.com/wouterdamman/home-finance/compare/v1.7.2...v1.7.3) (2026-07-19)


### Bug Fixes

* **helm:** auto-derive DB and OIDC CiliumNetworkPolicy egress ([8a211e2](https://github.com/wouterdamman/home-finance/commit/8a211e21dd09ba60dff4b840550eff43eadca741))

## [1.7.2](https://github.com/wouterdamman/home-finance/compare/v1.7.1...v1.7.2) (2026-07-19)


### Bug Fixes

* **helm:** support imagePullSecrets for private image repositories ([a40420f](https://github.com/wouterdamman/home-finance/commit/a40420f37867ea2e0568d957b0d9e441377c4f61))

## [1.7.1](https://github.com/wouterdamman/home-finance/compare/v1.7.0...v1.7.1) (2026-07-19)


### Bug Fixes

* **api:** stop swallowing Scan/Exec/rows.Err() across httpapi handlers ([e4847ba](https://github.com/wouterdamman/home-finance/commit/e4847ba8d44091bb7590251299e0ec12ae5d7fac))
* **auth:** purge expired sessions on a ticker ([10f8934](https://github.com/wouterdamman/home-finance/commit/10f8934790a17b25f1be8b17e87000c10bf6bfee))
* **import:** wrap XLSX import in one transaction ([2a8bb22](https://github.com/wouterdamman/home-finance/commit/2a8bb220573f1eaaf1fb6637fb9b1dc3f171fca2))
* **periods:** reject close when year locked or next period closed ([36bfa94](https://github.com/wouterdamman/home-finance/commit/36bfa94134b3835bad964c45aadc815405f5d2f3))
* **settings:** debounce audit-log filter inputs ([127927e](https://github.com/wouterdamman/home-finance/commit/127927ef2306fedfd6cfb810513896759bd44389))
* **trends:** don't clip negative surplus to zero on charts ([69c89c9](https://github.com/wouterdamman/home-finance/commit/69c89c9c311e76e73d8a4235de883f2bd9813634))

## [1.7.0](https://github.com/wouterdamman/home-finance/compare/v1.6.0...v1.7.0) (2026-07-19)


### Features

* **helm:** support Gateway API, ESO, external Postgres as default stack ([#46](https://github.com/wouterdamman/home-finance/issues/46)) ([7ee30de](https://github.com/wouterdamman/home-finance/commit/7ee30dea4bf2b46424b8667ad82cf87c78533882))

## [1.6.0](https://github.com/wouterdamman/home-finance/compare/v1.5.0...v1.6.0) (2026-07-18)


### Features

* **income:** add income-itemizing for sources like BD/MTC (H) ([a3149b1](https://github.com/wouterdamman/home-finance/commit/a3149b1aa34b53b7d5bb0e3f456d01291c1f4d52))
* **settings:** selectable chart color palette + fix table/chart height mismatch ([90cf9a2](https://github.com/wouterdamman/home-finance/commit/90cf9a223a132dba59f65fe9866e4da236a29869))
* **trends:** 2D widget resize (width x height) with HA-style picker ([2c4c307](https://github.com/wouterdamman/home-finance/commit/2c4c307454562fcdaf7531d14e0ae41144fb138c))
* **trends:** add category spend trends over months (C2) ([4aaa53a](https://github.com/wouterdamman/home-finance/commit/4aaa53ae5a06272dd85060bc4efcf099f66c15c3))
* **trends:** add month-vs-month comparison widget ([1965327](https://github.com/wouterdamman/home-finance/commit/1965327179685f39e38ad4be57ea977f766dee25))
* **trends:** drop Compare-years widget, add multi-category + widget delete + month-compare filtering ([c9b9d14](https://github.com/wouterdamman/home-finance/commit/c9b9d14b5cc9d630a1da2edf09c5cdb4298d799d))
* **trends:** rebuild as a modular, Power BI-like dashboard ([7fbbba5](https://github.com/wouterdamman/home-finance/commit/7fbbba5476613d079153c5cfba63c132f31237be))
* **trends:** replace category-widget clutter with 4 focused comparisons ([fe3af08](https://github.com/wouterdamman/home-finance/commit/fe3af0806421218539ec5d0a43fb1a2e6e8c3c90))
* **trends:** rework C2 into a cross-year Trends nav page ([b930811](https://github.com/wouterdamman/home-finance/commit/b93081164abcdab868ca3803fcc56a0994e58755))
* **trends:** richer default dashboard + multi-period month compare ([e2d34ba](https://github.com/wouterdamman/home-finance/commit/e2d34ba4fdf5e8e4888feb1f84f1e695d4c63367))


### Bug Fixes

* **import:** split ResetMaster into separate statements per pgx v5 ([01bf05b](https://github.com/wouterdamman/home-finance/commit/01bf05b4da1e18413dc6d942c7835f9be29d2f86))
* **periods:** check DB/parse errors in handleReplaceSplits ([963a6a0](https://github.com/wouterdamman/home-finance/commit/963a6a0f10da6023b73a41c28620b5b0cf959180))
* **periods:** use itemized-income totals everywhere, not plain SUM(amount_cents) ([ec757e2](https://github.com/wouterdamman/home-finance/commit/ec757e2b4a622a0add4090810c65457110384caf))
* **trends:** compact y-axis tick labels so they stop bleeding past the card ([7b0e57a](https://github.com/wouterdamman/home-finance/commit/7b0e57a5249c6e4b98526494b268a559ac81c45c))
* **trends:** replace MultiSelect overlay with a small-multiples dashboard ([422892e](https://github.com/wouterdamman/home-finance/commit/422892efb12ed7ad6d0fe37e3e22fc49bb9b8e88))
* **trends:** replace recharts legend with a plain Mantine legend ([4063d07](https://github.com/wouterdamman/home-finance/commit/4063d0717f93b627a1a20d1e90f069e0044bb962))
* **trends:** validate persisted widget config shape, not just type ([b278f7b](https://github.com/wouterdamman/home-finance/commit/b278f7b308ef8e1708a3bae678ebb8b8a2cc88c3))
* **trends:** y-axis overflow on remaining widgets + add year-dashboard-style chart option ([616ab20](https://github.com/wouterdamman/home-finance/commit/616ab20bdd8c06ad9717a50a91762aad04179d21))
* **trends:** year selector only listed years with data, not every registered year ([061210d](https://github.com/wouterdamman/home-finance/commit/061210d32425ac366a538faade581bec5bd0a273))

## [1.5.0](https://github.com/wouterdamman/home-finance/compare/v1.4.0...v1.5.0) (2026-07-17)


### Features

* **audit-log:** paginated/filterable UI + optional S3 export (G2) ([0203110](https://github.com/wouterdamman/home-finance/commit/02031107aa5a81772c13abfc75fbbad4d9a44129))
* **export:** add Excel export for a year or selected months ([0be8035](https://github.com/wouterdamman/home-finance/commit/0be80355c3fe887d0db38bdc35d1e07c63565a71))
* **import:** add Excel import for legacy Fam_Finance files ([a6d3dba](https://github.com/wouterdamman/home-finance/commit/a6d3dba4d4bfdb04fde31d782d2c89c65f05675e))
* **import:** also accept the app's own export format, fix two latent bugs ([ed9ffbf](https://github.com/wouterdamman/home-finance/commit/ed9ffbfca8a1bafe1f09196a8ec28684c01d1f19))
* **nav:** collapsible sidebar with peek/pin, tidy Settings tab bar ([fa086cd](https://github.com/wouterdamman/home-finance/commit/fa086cdce9ac1f4f84920fcddb15da0a9bd331dc))
* **rbac:** add admin/user roles, profile, and avatar upload (G1) ([3c0c8a3](https://github.com/wouterdamman/home-finance/commit/3c0c8a3c43e43e02b844404c225b3a6abde3f0bb))


### Bug Fixes

* **auth:** bootstrap dev admin user eagerly, not only on login ([10bc294](https://github.com/wouterdamman/home-finance/commit/10bc294ff521ec86f7ffb6033d4ea3440b99494c))

## [1.4.0](https://github.com/wouterdamman/home-finance/compare/v1.3.0...v1.4.0) (2026-07-16)


### Features

* **api:** add OpenAPI spec with Scalar API reference UI ([4c78628](https://github.com/wouterdamman/home-finance/commit/4c7862859108735d9376ea0ed19292556076e5a5))
* **mobile:** redesign UI for mobile as Apple-style app, not a shrunk desktop ([989e18c](https://github.com/wouterdamman/home-finance/commit/989e18c501c3f38dc373ec03a2429db250b9f83a))
* **pots:** add savings goals with target amount and target date ([d1842b9](https://github.com/wouterdamman/home-finance/commit/d1842b9fcd643df81bee89e563a72e20234bc580))
* **pwa:** make app installable as a PWA ([c787875](https://github.com/wouterdamman/home-finance/commit/c787875b1292b44137ba855191dec444778b3232))
* **settings:** move add-item forms into a modal, add search and column sort ([37e4336](https://github.com/wouterdamman/home-finance/commit/37e43362120060593023ea508c216152a0c202e6))
* **settings:** move theme and language controls into Settings ([7856713](https://github.com/wouterdamman/home-finance/commit/78567131a81064e8b700a1c86ac333bafb37036e))
* **ui:** custom theme, tabler icons, empty states, save notifications ([b34c682](https://github.com/wouterdamman/home-finance/commit/b34c68290120efb5b4f82aed93929da4977f9cae))


### Bug Fixes

* **mobile:** make Settings a real drill-down menu, not one long page ([9bd6541](https://github.com/wouterdamman/home-finance/commit/9bd65411129162b6cc54136d628764d7667590a8))
* **mobile:** stop nesting a Progress bar inside a Text paragraph ([7246404](https://github.com/wouterdamman/home-finance/commit/7246404523c5a3c18a24ff810b1da6492374324f))
* **month:** deep-link budget-line clicks and guard the itemized toggle ([814f1dd](https://github.com/wouterdamman/home-finance/commit/814f1dd625ee6b8623d9a779458f7618f26e9918))
* **settings:** align edit-mode inputs with their column headers ([727c465](https://github.com/wouterdamman/home-finance/commit/727c46522a858f1757b5772486a4e1e70e9d2252))

## [1.3.0](https://github.com/wouterdamman/home-finance/compare/v1.2.0...v1.3.0) (2026-07-16)


### Features

* **pots:** add savings pots ledger page and period allocation editor ([dde9571](https://github.com/wouterdamman/home-finance/commit/dde9571cc505f19e2ffc2fe12c4130477c9bba5f))
* **ui:** add year charts, budget progress bars and transaction dates ([e1e91be](https://github.com/wouterdamman/home-finance/commit/e1e91bea8f3ed6b5f058d757132e8a57303f1601))
* **years:** make years explicit instead of a hardcoded sliding window ([5a1c830](https://github.com/wouterdamman/home-finance/commit/5a1c830aecbc0e67c20df149917a5ab619433bda))


### Bug Fixes

* **api:** keep carryover pot percentage authoritative on split save ([c21a159](https://github.com/wouterdamman/home-finance/commit/c21a159b89a9441ca0efad1b36eb961606796974))
* **dev:** correct postgres 18 volume mount path in docker-compose ([16babf9](https://github.com/wouterdamman/home-finance/commit/16babf9ae1ad21b39884244b9cb1f40b0f29e11f))
* **ui:** fix category tab label wrapping in transactions view ([4782efe](https://github.com/wouterdamman/home-finance/commit/4782efe0d041a98670c154c5731f924aa95d7706))
* **ui:** mount ModalsProvider so confirm dialogs actually render ([d3bfd49](https://github.com/wouterdamman/home-finance/commit/d3bfd496ff2e9f16c16ec9c417d41d49aeae3659))

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
