# Changelog

## [1.17.0](https://github.com/wouterdamman/home-finance/compare/v1.16.0...v1.17.0) (2026-08-24)


### Features

* allow deleting an empty category or income source from a running month ([e78ac1d](https://github.com/wouterdamman/home-finance/commit/e78ac1dc0101943be73ee3585e01a8f8c5b73e28))

## [1.16.0](https://github.com/wouterdamman/home-finance/compare/v1.15.1...v1.16.0) (2026-08-23)


### Features

* **month:** itemized autofill copy-forward, editable transactions/pots, live split editor ([eb1b0ac](https://github.com/wouterdamman/home-finance/commit/eb1b0ac1104b95bf58bb5ce79e5f96903af09759))


### Bug Fixes

* **deps:** override transitive nanoid to patched 3.3.18 (CVE-2026-67213) ([b7146bc](https://github.com/wouterdamman/home-finance/commit/b7146bccd9649858df32dc7b00233109af174cd1))
* **deps:** update module github.com/go-chi/chi/v5 to v5.3.2 ([#97](https://github.com/wouterdamman/home-finance/issues/97)) ([d8abaff](https://github.com/wouterdamman/home-finance/commit/d8abaff184819011adfc991281b9fa995ca563de))
* **tests:** update integration tests for autofill/itemized behavior change ([a49472e](https://github.com/wouterdamman/home-finance/commit/a49472e1ea94765890f4ad3f6c0f883e40e7f939))

## [1.15.1](https://github.com/wouterdamman/home-finance/compare/v1.15.0...v1.15.1) (2026-08-19)


### Bug Fixes

* **deps:** update module github.com/minio/minio-go/v7 to v7.3.0 ([#90](https://github.com/wouterdamman/home-finance/issues/90)) ([2661c45](https://github.com/wouterdamman/home-finance/commit/2661c45a9081974684bc2118c7ce430bfd770a84))

## [1.15.0](https://github.com/wouterdamman/home-finance/compare/v1.14.0...v1.15.0) (2026-08-05)


### Features

* **migrate-avatars:** add tool to copy filesystem avatars into Postgres ([e249317](https://github.com/wouterdamman/home-finance/commit/e249317c369bc8f4d3a2c2548c239d8a18e8fe04))


### Bug Fixes

* **avatars:** remove filesystem PVC backend, Postgres-only now ([5db5c36](https://github.com/wouterdamman/home-finance/commit/5db5c361c5bfc534464817b2036b61b37c65aba5))
* **frontend:** description autocomplete only shows curated presets ([6087be1](https://github.com/wouterdamman/home-finance/commit/6087be1418f37e91eb7a258acc140852e6a9197d))
* **helm:** use Recreate rollout strategy to avoid RWO volume race ([89f5ea9](https://github.com/wouterdamman/home-finance/commit/89f5ea9200f05b87ec9e439d29613285463087b2))

## [1.14.0](https://github.com/wouterdamman/home-finance/compare/v1.13.2...v1.14.0) (2026-08-04)


### Features

* **settings:** curated description presets per category/income source ([8c497f2](https://github.com/wouterdamman/home-finance/commit/8c497f23a94cddbca6c8b14e563540f3bee770af))


### Bug Fixes

* **categories:** make autofillActual and isItemized mutually exclusive ([70ba3e3](https://github.com/wouterdamman/home-finance/commit/70ba3e328514b12e3202b651e65fe62e1c210dcc))
* **deps:** update dependency typescript to v7 ([#56](https://github.com/wouterdamman/home-finance/issues/56)) ([4a99b63](https://github.com/wouterdamman/home-finance/commit/4a99b634702b99872a404c16c4c4f3ec1a203bc6))

## [1.13.2](https://github.com/wouterdamman/home-finance/compare/v1.13.1...v1.13.2) (2026-07-23)


### Bug Fixes

* **income:** link ad-hoc income entries to an income source ([c36e3cc](https://github.com/wouterdamman/home-finance/commit/c36e3cc824ef590a4c272d8387551c981f4e20e5))
* **periods:** apply category/income template when closing a period ([b17753c](https://github.com/wouterdamman/home-finance/commit/b17753c6175cf30b07469062468ac744663965d7))

## [1.13.1](https://github.com/wouterdamman/home-finance/compare/v1.13.0...v1.13.1) (2026-07-23)


### Bug Fixes

* **ui:** improve mobile list row alignment, add Trends to bottom nav ([10adf58](https://github.com/wouterdamman/home-finance/commit/10adf588c8c00786a29fdcf2c467a5721508ab10))

## [1.13.0](https://github.com/wouterdamman/home-finance/compare/v1.12.0...v1.13.0) (2026-07-23)


### Features

* **ui:** sort income, expenses, and pot splits within a month ([8f5d039](https://github.com/wouterdamman/home-finance/commit/8f5d03999097d4c2fd706c91b7a1916c11d0f25c))


### Bug Fixes

* **deps:** pin fast-uri to patched version ([9514d65](https://github.com/wouterdamman/home-finance/commit/9514d65cf7a6314c74d5d3399e64be65dd5131c2))
* **deps:** update module github.com/pressly/goose/v3 to v3.27.3 ([13042e6](https://github.com/wouterdamman/home-finance/commit/13042e67cc097bd9795338af3b4cf04a98c68aba))

## [1.12.0](https://github.com/wouterdamman/home-finance/compare/v1.11.0...v1.12.0) (2026-07-23)


### Features

* **api:** carry itemized income-transaction line items into new periods ([bdee6ab](https://github.com/wouterdamman/home-finance/commit/bdee6ab146861505a6dc2a7408fa49a92965325d))

## [1.11.0](https://github.com/wouterdamman/home-finance/compare/v1.10.5...v1.11.0) (2026-07-21)


### Features

* **api:** add hard-delete for income sources ([e86e7d9](https://github.com/wouterdamman/home-finance/commit/e86e7d91d634246374f0e2515a5727a48ee5ee42))
* **mobile:** lead year overview with balance, not surplus ([ffc9225](https://github.com/wouterdamman/home-finance/commit/ffc9225676e7bb3a3de8be7452701e0d79eeb634))


### Bug Fixes

* **pwa:** exclude /api/ from service worker navigation fallback ([536c3fc](https://github.com/wouterdamman/home-finance/commit/536c3fc768c310f0367a02199632932642d0fc20))
* **ui:** show budget-line actual/target amount inline ([0a579f5](https://github.com/wouterdamman/home-finance/commit/0a579f59bedfc67109b1cebc06f71c5ca535c968))

## [1.10.5](https://github.com/wouterdamman/home-finance/compare/v1.10.4...v1.10.5) (2026-07-21)


### Bug Fixes

* **helm:** stop deleting+recreating the namespace on every ArgoCD sync ([edb26a6](https://github.com/wouterdamman/home-finance/commit/edb26a6059a0a111321056c0d3b528674ad69804))

## [1.10.2](https://github.com/wouterdamman/home-finance/compare/v1.10.1...v1.10.2) (2026-07-21)


### Bug Fixes

* **auth:** register time.Time with gob so reauth session commits succeed ([b67f201](https://github.com/wouterdamman/home-finance/commit/b67f2016bdf576012fd00f52d40813ae516f78c4))

## [1.10.1](https://github.com/wouterdamman/home-finance/compare/v1.10.0...v1.10.1) (2026-07-21)


### Bug Fixes

* **helm:** orphan ExternalSecret's target Secret to survive upgrade churn ([bd02dd7](https://github.com/wouterdamman/home-finance/commit/bd02dd752e7bb1873dc337205d284f3601b9f859))
* **helm:** set fsGroup on pod so PV-backed avatar storage is writable ([fe3f02b](https://github.com/wouterdamman/home-finance/commit/fe3f02b778e086edc04fc0ac3df30d20254bfe26))
* **helm:** stop namespace pre-upgrade hook from wiping the namespace ([581fba0](https://github.com/wouterdamman/home-finance/commit/581fba0997769c6f2a61800f5d5f2286f8340989))


### CI/CD

* **release:** prune old GHCR package versions after each release, keep last 4 ([cc50431](https://github.com/wouterdamman/home-finance/commit/cc504310de1a35452ad0c784c2d52ce7a7e4546d))

## [1.10.0](https://github.com/wouterdamman/home-finance/compare/v1.9.0...v1.10.0) (2026-07-21)


### Features

* **avatar:** add optional filesystem storage backend ([6b30f9c](https://github.com/wouterdamman/home-finance/commit/6b30f9c7b6b6a572d34753fe0affc03eafba00ee))
* **avatar:** add pluggable storage interface + Postgres-bytea backend ([1101abd](https://github.com/wouterdamman/home-finance/commit/1101abde301026a77aa6acaf8b13eaf7c027a1cf))
* **avatar:** switch upload/get/list handlers to pluggable storage ([9858098](https://github.com/wouterdamman/home-finance/commit/9858098687899103e59f6fa0ee2438ca76736e8f))
* **avatar:** wire pluggable storage into server config ([4c64c00](https://github.com/wouterdamman/home-finance/commit/4c64c00351a6a531d44de497503fc52c0f772afe))
* **db:** add avatar_data/avatar_content_type columns for bytea avatar storage ([d3740e7](https://github.com/wouterdamman/home-finance/commit/d3740e74057be1182fcefa739b9d46edbe000f51))
* **helm:** optional PersistentVolume-backed avatar storage ([3b531b2](https://github.com/wouterdamman/home-finance/commit/3b531b2553a972b9445d3fdd8c7d53cd9b6c9160))


### Bug Fixes

* **header:** show avatar next to user name ([fbf74be](https://github.com/wouterdamman/home-finance/commit/fbf74be565f816259e06cb0f6fac3037626a144f))
* **import:** register year in years table on period creation ([e905aea](https://github.com/wouterdamman/home-finance/commit/e905aeae0bcfbacb9bd61617c05b15a16acbff2c))
* **mobile:** prevent SwipeableList crash on falsy conditional children ([c21249e](https://github.com/wouterdamman/home-finance/commit/c21249e74266ac4a2f8f918362b1fb66e80a037d))
* **settings:** match avatar upload accept/size limit to backend ([6722092](https://github.com/wouterdamman/home-finance/commit/6722092174a79dabd2e5585d7a67f4c69ad655ef))

## [1.9.0](https://github.com/wouterdamman/home-finance/compare/v1.8.0...v1.9.0) (2026-07-20)


### Features

* **api:** categories.autofillActual field in list/create/update ([33a91e3](https://github.com/wouterdamman/home-finance/commit/33a91e34e91fafe0f6a99e4c1991cd195fe5834b))
* **db:** add categories.autofill_actual + budget_lines.target_cents_at_close ([bf8f094](https://github.com/wouterdamman/home-finance/commit/bf8f094497cdd9733ae0824c703d5777db9071e5))
* **frontend:** budgetLine.targetCents + category.autofillActual types ([b989f56](https://github.com/wouterdamman/home-finance/commit/b989f567c39db9713dc7ae0120c31191283006b9))
* **month:** decouple actual spend from budget target ([9624e40](https://github.com/wouterdamman/home-finance/commit/9624e40d67248e0185aa5bad46a5b1c66b1de764))
* **month:** show budget target vs actual spend separately ([a2c918a](https://github.com/wouterdamman/home-finance/commit/a2c918a81cda3210e3ed53c1dfd0e29dd77d478e))
* **settings:** autofill-actual toggle for categories ([5a2d066](https://github.com/wouterdamman/home-finance/commit/5a2d066848dd73e6a686fdc7e065e0e70aed94c9))


### Bug Fixes

* **settings:** add missing column header for autofill-actual toggle ([158a9f8](https://github.com/wouterdamman/home-finance/commit/158a9f882c3aefd37a699b5bf7db93373f8794ae))


### CI/CD

* **release:** allow manually re-running the image build via workflow_dispatch ([7d178d4](https://github.com/wouterdamman/home-finance/commit/7d178d42bd8d632e74a1812ec10048fc871df9fa))

## [1.8.0](https://github.com/wouterdamman/home-finance/compare/v1.7.6...v1.8.0) (2026-07-20)


### Features

* **api:** CRUD endpoints for category_aliases ([f3eccd5](https://github.com/wouterdamman/home-finance/commit/f3eccd56204a3c063c4771ae08a67be6b2d91e4b))
* **auth:** replace DELETE_PASSWORD PIN with Authentik step-up reauth ([aaeaa9e](https://github.com/wouterdamman/home-finance/commit/aaeaa9ecf1d7a2c0940467bf74efc292cc1f5610))
* **categories:** parent/child invariants + import alias resolution ([4487521](https://github.com/wouterdamman/home-finance/commit/448752124d4d2f4f7198a37db1928dd89b9348e0))
* **db:** add category parent_id, category_rollup view, category_aliases ([7c49aa8](https://github.com/wouterdamman/home-finance/commit/7c49aa8a36eb76923987bb9eca93868fa4f8a3eb))
* **db:** fold child-category transactions into parent budget line totals ([4596f9e](https://github.com/wouterdamman/home-finance/commit/4596f9e479805d4ac4f1140b40bf6c30b80021fb))
* **frontend:** category hierarchy UI + import alias management ([b2954a0](https://github.com/wouterdamman/home-finance/commit/b2954a0b39c656ea65382459b50d1e236c02082b))
* **frontend:** pop up Authentik reauth instead of a PIN prompt ([05df7fd](https://github.com/wouterdamman/home-finance/commit/05df7fd5a2ec5d48dcf7677a5460febdef9c175a))
* **month:** make a budget line's amount editable after creation ([d0e327c](https://github.com/wouterdamman/home-finance/commit/d0e327cec008313de116a4ee4a3aed633c9c6e57))


### Bug Fixes

* **import:** stop silently dropping sheets with non-canonical names ([567956f](https://github.com/wouterdamman/home-finance/commit/567956fb1d75cc1839af3c46d2eb346bb3f26dd5))

## [1.7.6](https://github.com/wouterdamman/home-finance/compare/v1.7.5...v1.7.6) (2026-07-20)


### Bug Fixes

* **release:** drop release-type input so release-please reads its config file ([4436d21](https://github.com/wouterdamman/home-finance/commit/4436d21215207741f1e5db0faff467aac4d346d1))
* **release:** use generic extra-files updater to avoid rewriting values.yaml ([1b64e2e](https://github.com/wouterdamman/home-finance/commit/1b64e2eec7b9d4713a585364b76024e2ef687906))

## [1.7.5](https://github.com/wouterdamman/home-finance/compare/v1.7.4...v1.7.5) (2026-07-20)


### Bug Fixes

* **release:** bundle helm chart version bump into the release-please tag ([82cbe40](https://github.com/wouterdamman/home-finance/commit/82cbe40031f0b31abf7e22bde37117257a0c3741))

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
