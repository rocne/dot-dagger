# Changelog

All notable changes are documented here. This file is maintained automatically
by [release-please](https://github.com/googleapis/release-please) from
[Conventional Commit](https://www.conventionalcommits.org/) messages.

Releases prior to the adoption of release-please (≤ v0.5.5) are recorded on the
[GitHub releases page](https://github.com/rocne/dot-dagger/releases).

## [0.10.2](https://github.com/rocne/dot-dagger/compare/v0.10.1...v0.10.2) (2026-06-22)


### Bug Fixes

* correct misleading get-os help text ([#177](https://github.com/rocne/dot-dagger/issues/177)) ([e1124b4](https://github.com/rocne/dot-dagger/commit/e1124b4a2dc64936778f1ad372a18b43ae1104d0))

## [0.10.1](https://github.com/rocne/dot-dagger/compare/v0.10.0...v0.10.1) (2026-06-17)


### Bug Fixes

* **dist:** use dot-dagger as the package name on all channels ([#172](https://github.com/rocne/dot-dagger/issues/172)) ([62d4d9c](https://github.com/rocne/dot-dagger/commit/62d4d9c24f812829a2450ed94cba9b5eb2c39222))

## [0.10.0](https://github.com/rocne/dot-dagger/compare/v0.9.1...v0.10.0) (2026-06-17)


### Features

* **dist:** publish deb/rpm to Cloudsmith apt/dnf repo ([#170](https://github.com/rocne/dot-dagger/issues/170)) ([123d455](https://github.com/rocne/dot-dagger/commit/123d45537d238db32eb72d994b214ca1eea72679))


### Bug Fixes

* **cli:** make teardown/unapply/check pipeline preamble non-interactive ([#166](https://github.com/rocne/dot-dagger/issues/166)) ([75cb795](https://github.com/rocne/dot-dagger/commit/75cb79567f768b8b4db1f7d00343489a398d302c))
* harden broad-audit LOW findings (silent drops, scan caps, error swallows) ([#168](https://github.com/rocne/dot-dagger/issues/168)) ([bbd8ca4](https://github.com/rocne/dot-dagger/commit/bbd8ca4bd07cf95f1d8fb78cbd6f306e769f40a7))

## [0.9.1](https://github.com/rocne/dot-dagger/compare/v0.9.0...v0.9.1) (2026-06-16)


### Bug Fixes

* **annotation:** recover unterminated-paren key and align Scan/Write ([#162](https://github.com/rocne/dot-dagger/issues/162)) ([c3616b9](https://github.com/rocne/dot-dagger/commit/c3616b9ec4aa46f55fa937dde8bf18854b46a9ce))
* **cli:** add actionable hints to bundle and unapply errors ([#149](https://github.com/rocne/dot-dagger/issues/149)) ([9f2e2f1](https://github.com/rocne/dot-dagger/commit/9f2e2f1f913464cdcddc70a76eb5fff37655afbd))
* **cli:** deterministic bundle env order and dry-run compose preview ([#165](https://github.com/rocne/dot-dagger/issues/165)) ([61c1eec](https://github.com/rocne/dot-dagger/commit/61c1eecdb2d84c55ba10b3231f68230e5ad3392d))
* **cli:** teardown recovery, apply resumability docs, and error hints ([#160](https://github.com/rocne/dot-dagger/issues/160)) ([aeb91ab](https://github.com/rocne/dot-dagger/commit/aeb91ab40d963d435654880a94faf3b3392b627b))
* **fileutil:** single-quote brace-expansion characters in ShellQuote ([#164](https://github.com/rocne/dot-dagger/issues/164)) ([1b68bdd](https://github.com/rocne/dot-dagger/commit/1b68bdd206a4218fc86597aeae807f6fb620be49))
* **install:** fail closed when no checksum tool is available ([#148](https://github.com/rocne/dot-dagger/issues/148)) ([e20d6f1](https://github.com/rocne/dot-dagger/commit/e20d6f1a05b20fcfa0401c62e8ff6541e85b60c2))
* **node:** keep logical name when prefix-strip would empty it ([#152](https://github.com/rocne/dot-dagger/issues/152)) ([c56639f](https://github.com/rocne/dot-dagger/commit/c56639fda58249f77557c67192aacecc4502a920))
* **pipeline:** harden node naming, [@after](https://github.com/after) prefix, and link/compose edges ([#146](https://github.com/rocne/dot-dagger/issues/146)) ([64afb2d](https://github.com/rocne/dot-dagger/commit/64afb2d7c0572beb7c7d228c0de49ce35a8ae2e8))
* **pipeline:** honor compose:true shorthand and separate fragments ([#161](https://github.com/rocne/dot-dagger/issues/161)) ([e684c53](https://github.com/rocne/dot-dagger/commit/e684c53e6f6273104f0ff896ade4b48b3fe37e07))
* **predicate:** bound parser nesting depth ([#154](https://github.com/rocne/dot-dagger/issues/154)) ([15e19c5](https://github.com/rocne/dot-dagger/commit/15e19c5fd5fdfa0eedf444c4109b4a20637c09bb))
* **predicate:** make AND/OR commutative under missing env keys ([#145](https://github.com/rocne/dot-dagger/issues/145)) ([214f7e0](https://github.com/rocne/dot-dagger/commit/214f7e0b55d9c4b6c313910561938f012e9b8f2c))
* **setup:** atomic RC rewrite and structural source-line match ([#163](https://github.com/rocne/dot-dagger/issues/163)) ([ed7dda7](https://github.com/rocne/dot-dagger/commit/ed7dda7347f03b3f26dc84e186ed9b5978fa7e33))

## [0.9.0](https://github.com/rocne/dot-dagger/compare/v0.8.0...v0.9.0) (2026-06-15)


### Features

* ship GPG-signed deb and rpm packages ([#142](https://github.com/rocne/dot-dagger/issues/142)) ([7533b2f](https://github.com/rocne/dot-dagger/commit/7533b2f4aaf46d9617cfda0c95174b804f6921d9))

## [0.8.0](https://github.com/rocne/dot-dagger/compare/v0.7.0...v0.8.0) (2026-06-15)


### Features

* embed docs into the binary as `dotd docs --full` ([#140](https://github.com/rocne/dot-dagger/issues/140)) ([faa0368](https://github.com/rocne/dot-dagger/commit/faa0368977e8ed75ea3f09270429566da5545d17))

## [0.7.0](https://github.com/rocne/dot-dagger/compare/v0.6.1...v0.7.0) (2026-06-15)


### Features

* sign releases with cosign + SLSA provenance ([#137](https://github.com/rocne/dot-dagger/issues/137)) ([5cf4a26](https://github.com/rocne/dot-dagger/commit/5cf4a2676b03fb76e54ce67638f55775aa14ade4))

## [0.6.1](https://github.com/rocne/dot-dagger/compare/v0.6.0...v0.6.1) (2026-06-14)


### Bug Fixes

* tolerate legacy/corrupt config.yaml in path resolution ([#131](https://github.com/rocne/dot-dagger/issues/131)) ([b7acefa](https://github.com/rocne/dot-dagger/commit/b7acefaaf771fa558d1d8d2748def164300d496c))

## [0.6.0](https://github.com/rocne/dot-dagger/compare/v0.5.5...v0.6.0) (2026-06-14)


### Features

* **dotd:** report commit and build date in --version ([#126](https://github.com/rocne/dot-dagger/issues/126)) ([82e088d](https://github.com/rocne/dot-dagger/commit/82e088d68bba04277cb5544ad1d6ad6b0a21952a))
