# Changelog

## [1.2.0](https://github.com/cskiller24/querylex/compare/v1.1.0...v1.2.0) (2026-06-13)


### Features

* **260613-0ym:** add generate-encryption and rotate-encryption CLI commands, enhance workspace-stats connectivity ([b13fcda](https://github.com/cskiller24/querylex/commit/b13fcda7b4c002a60eac7e33c2c99e99a62be133))
* **260613-0ym:** replace machine-ID key derivation with stored generated AES-256 key ([62f85cc](https://github.com/cskiller24/querylex/commit/62f85ccc636e0ecf2cfaed846ad3bdffb2e36290))
* add installation script and update binary check in querylex ([5cc740a](https://github.com/cskiller24/querylex/commit/5cc740a01a01560dbe0a0fc009e506b46db7d8f1))
* **memory-improve:** add bigram tokenization to keyword index and search ([79baf69](https://github.com/cskiller24/querylex/commit/79baf69a0967a794bedbd3869160963910e41555))
* **memory-improve:** add FTS5 virtual table and fallback search ([e706caa](https://github.com/cskiller24/querylex/commit/e706caab2ed6934dfe58fb339884676a788e38ed))
* **memory-improve:** add sqlStructureScore with 0.15 weight ([37b307e](https://github.com/cskiller24/querylex/commit/37b307e65413790de47827f2f488531e539dd79d))
* **memory-improve:** add token frequency map and skip high-frequency tokens ([4db4f58](https://github.com/cskiller24/querylex/commit/4db4f58e30503052965ed090bd9beddfab914cb3))
* **memory-improve:** decay entries older than 90 days from search results ([921a4e3](https://github.com/cskiller24/querylex/commit/921a4e36585fe9c96011a4301b6cdb81adac84b1))
* **memory-improve:** lower memory match threshold from 0.86 to 0.60 ([82c7121](https://github.com/cskiller24/querylex/commit/82c7121975809a4a70f8920737b3d705f9ab7a51))
* **quick-260613-21y:** add interactive delete-db/use-db, human-readable list-dbs ([7772555](https://github.com/cskiller24/querylex/commit/777255504965d8d622d79cdd6c665e7de6efbea9))
* **quick-260613-21y:** consolidate encryption to 'encrypt' command, add non-interactive add-db flags ([ff22609](https://github.com/cskiller24/querylex/commit/ff226091e65b15f588c256f2ced97df4f4efa583))
* **state:** add UpdateDatabase and DeleteDatabase to WorkspaceStore ([f19534c](https://github.com/cskiller24/querylex/commit/f19534c0c94f070f4e893558e7fcecfa243a48f0))
* **stats:** add database connectivity check to workspace-stats ([2fe1553](https://github.com/cskiller24/querylex/commit/2fe1553217033b308373361f8330140d93aeb992))
* **task3:** create machinekey.go with cross-platform machine ID derivation ([e8f7735](https://github.com/cskiller24/querylex/commit/e8f77351814af50a058753a4f55afe36e2849aea))


### Bug Fixes

* remove addEmbeddingsWarning dead code and embedding references ([feea744](https://github.com/cskiller24/querylex/commit/feea744341926d9cc2e002ffbc34824ddde27cd0))
* update querylex bin path in package.json ([1ec9f0c](https://github.com/cskiller24/querylex/commit/1ec9f0c2ecf65c2a0fb9caa0274f87ffc9418423))


### Documentation

* **260608-8e4:** comprehensive CLI documentation ([14b845d](https://github.com/cskiller24/querylex/commit/14b845d7c4caa1b32e4a38ccc401aaa2e8b9caf9))
* **260611-i6z:** pre-dispatch plan for Execute all improvements ([8a19bfa](https://github.com/cskiller24/querylex/commit/8a19bfa1d154943ad5d779958e8e6406daad18a2))
* **quick-260608-8e4:** CLI documentation ([10af9d0](https://github.com/cskiller24/querylex/commit/10af9d03cfd450c7bc06454a9219512ca3ae621e))
* **quick-260611-i6z:** Execute all improvements from new-improvements.md ([91ae159](https://github.com/cskiller24/querylex/commit/91ae159b8d965d48bec3950f686afad55ff3b6c5))
* **state:** record quick task 260607-v40 ([11e6e6f](https://github.com/cskiller24/querylex/commit/11e6e6f2708e16c631c7c6db2e590fa45b14d5bb))
* **state:** record quick task 260611-i6z ([3c58a8b](https://github.com/cskiller24/querylex/commit/3c58a8b21f2c367f1bbcbe3e132cc44327a0772e))
* **state:** record quick task 260613-0ym ([a38647c](https://github.com/cskiller24/querylex/commit/a38647c30d2bb7407bd315a2578e48a532495ddb))


### Miscellaneous Chores

* **1.x:** release 1.2.0 ([03eaed3](https://github.com/cskiller24/querylex/commit/03eaed3ed08223e4bb675a9d6ae9dfe3b89ab452))
* **1.x:** release 1.2.0 ([66f1898](https://github.com/cskiller24/querylex/commit/66f1898e99e3f460072e34ffc7198dd4de05a8a2))
* **1.x:** release 1.2.0 ([55c4646](https://github.com/cskiller24/querylex/commit/55c4646fd55ebaa1784108da2b10725c957b088e))
* **1.x:** release 1.2.0 ([aa09265](https://github.com/cskiller24/querylex/commit/aa092651e337dcf361275d2c8b839884addc6144))
* **1.x:** release 1.2.1 ([4108b93](https://github.com/cskiller24/querylex/commit/4108b937105623b8083d7784a7d1a9768d74d000))
* **1.x:** release 1.2.1 ([ac54815](https://github.com/cskiller24/querylex/commit/ac5481577b6eb26788947be9219720bc330f9f4b))
* **1.x:** release 1.2.2 ([2e7050d](https://github.com/cskiller24/querylex/commit/2e7050d07cc266a76f3c8286035b0bf9717295d0))
* **1.x:** release 1.2.2 ([6126c96](https://github.com/cskiller24/querylex/commit/6126c968791085d0bcab50a8388c80ed7bb962ef))
* add .npmignore file and remove unused files field from package.json ([3b26997](https://github.com/cskiller24/querylex/commit/3b269979d90a28a5a18d5ca828f3a3d37eeb9204))
* **hotfix:** Remove querylex completion scripts for Fish, PowerShell, and Zsh ([eaefb7e](https://github.com/cskiller24/querylex/commit/eaefb7ed44a3a381b417ba1291763f8fc2f361a3))
* **hotfix:** Remove querylex completion scripts for Fish, PowerShell, and Zsh ([c6154dd](https://github.com/cskiller24/querylex/commit/c6154ddb2ef68f7740ebb2b88001f016f371ca19))
* npm versioning ([19ec5b8](https://github.com/cskiller24/querylex/commit/19ec5b88a87fb7e1da23882ee2c062a54ab96b12))
* remove .npmignore file and update package.json to include files section ([24adc91](https://github.com/cskiller24/querylex/commit/24adc91a5eb8990da263fa7b1ef7609a00b3bec7))
* **task3:** remove passphrase.go ([bebfd4c](https://github.com/cskiller24/querylex/commit/bebfd4c664a68df7006f244f7c0dfcfaa9b624ed))


### Code Refactoring

* remove installation script and streamline querylex setup process lazy loading ([89040e8](https://github.com/cskiller24/querylex/commit/89040e879399fb17edc811abb7889528f4459d3b))
* **task3:** remove passphrase auto-unlock from preflight and related files ([a7d96f7](https://github.com/cskiller24/querylex/commit/a7d96f70464eed5c816af6571fe5a4bb205d9078))
* **task3:** remove passphrase prompt from run_adddb.go ([1c97a69](https://github.com/cskiller24/querylex/commit/1c97a69e3fa5bdf758f2cf1aa5ef830fbafc1738))
* **task3:** remove passphrase/scrypt from encrypted.go, use machine key ([6d50fd9](https://github.com/cskiller24/querylex/commit/6d50fd9f9bad0fcbd740bf75d49e9325ab5eccf2))


### Tests

* **task3:** remove passphrase-dependent tests from preflight_test.go ([d365609](https://github.com/cskiller24/querylex/commit/d36560924dfd2fdad144a8cd45fc1a9e50406341))
* **task3:** rewrite credential tests for passphrase-less operation ([e965761](https://github.com/cskiller24/querylex/commit/e9657611d15f5e9456fd96d2706fd0a603890dff))


### Continuous Integration

* add automated release pipeline for 1.x branch ([325a342](https://github.com/cskiller24/querylex/commit/325a34292f2cb83ed56e403c4c92953d10f8066a))
* add release-please manifest with current version 1.1 ([28a113b](https://github.com/cskiller24/querylex/commit/28a113b84a6567d6534c3e2b4584769f18494ab3))
* fix manifest to match latest release v1.1.0 ([33cea9a](https://github.com/cskiller24/querylex/commit/33cea9a368f6f402f54af35855d47a2469b6c78b))
* fix manifest version to semver 1.1.0 ([310b1c1](https://github.com/cskiller24/querylex/commit/310b1c13122c5ccd5171360065f49f75205975e1))
* harden release pipeline ([55d518a](https://github.com/cskiller24/querylex/commit/55d518a2c6d474b17a74beeb6792311c294cffba))
* re-add --provenance (package now exists on npm) ([70e15e7](https://github.com/cskiller24/querylex/commit/70e15e7e8aa75c9a41bf895e2d3d86dbda63dae6))
* remove --provenance for first publish (requires existing package) ([1d7df6e](https://github.com/cskiller24/querylex/commit/1d7df6e87bfec6a8775cd94c77efbdcbc5d43436))
* rename config to release-please-config.json (no dot prefix) ([c4f9b92](https://github.com/cskiller24/querylex/commit/c4f9b92ec4d04cfe6c25c4b876f70eed085e0e79))
* reset manifest to 1.0.0 after stale tag cleanup ([61d7cac](https://github.com/cskiller24/querylex/commit/61d7cac10c0ef2f5336b8edca41ad3ce41caf51f))
* simplify CI to single test job on ubuntu-latest ([6d52bc1](https://github.com/cskiller24/querylex/commit/6d52bc1cc4c9f0ed872e8fd5e445f7b97e1a93e3))
* switch npm publish to trusted publishing (OIDC) ([f17c135](https://github.com/cskiller24/querylex/commit/f17c13530158032200ef0ae1a3b0cb7c1d9f3859))
* use PAT instead of GITHUB_TOKEN for release-please to trigger downstream workflows ([5f16306](https://github.com/cskiller24/querylex/commit/5f1630668cb87fd768a2ea957a874101e16d9580))

## [1.2.2](https://github.com/cskiller24/querylex/compare/v1.2.1...v1.2.2) (2026-06-13)


### Bug Fixes

* update querylex bin path in package.json ([1ec9f0c](https://github.com/cskiller24/querylex/commit/1ec9f0c2ecf65c2a0fb9caa0274f87ffc9418423))


### Continuous Integration

* re-add --provenance (package now exists on npm) ([70e15e7](https://github.com/cskiller24/querylex/commit/70e15e7e8aa75c9a41bf895e2d3d86dbda63dae6))
* remove --provenance for first publish (requires existing package) ([1d7df6e](https://github.com/cskiller24/querylex/commit/1d7df6e87bfec6a8775cd94c77efbdcbc5d43436))

## [1.2.1](https://github.com/cskiller24/querylex/compare/v1.2.0...v1.2.1) (2026-06-13)


### Continuous Integration

* switch npm publish to trusted publishing (OIDC) ([f17c135](https://github.com/cskiller24/querylex/commit/f17c13530158032200ef0ae1a3b0cb7c1d9f3859))

## [1.2.0](https://github.com/cskiller24/querylex/compare/v1.1.0...v1.2.0) (2026-06-13)


### Features

* **260613-0ym:** add generate-encryption and rotate-encryption CLI commands, enhance workspace-stats connectivity ([b13fcda](https://github.com/cskiller24/querylex/commit/b13fcda7b4c002a60eac7e33c2c99e99a62be133))
* **260613-0ym:** replace machine-ID key derivation with stored generated AES-256 key ([62f85cc](https://github.com/cskiller24/querylex/commit/62f85ccc636e0ecf2cfaed846ad3bdffb2e36290))
* add installation script and update binary check in querylex ([5cc740a](https://github.com/cskiller24/querylex/commit/5cc740a01a01560dbe0a0fc009e506b46db7d8f1))
* **memory-improve:** add bigram tokenization to keyword index and search ([79baf69](https://github.com/cskiller24/querylex/commit/79baf69a0967a794bedbd3869160963910e41555))
* **memory-improve:** add FTS5 virtual table and fallback search ([e706caa](https://github.com/cskiller24/querylex/commit/e706caab2ed6934dfe58fb339884676a788e38ed))
* **memory-improve:** add sqlStructureScore with 0.15 weight ([37b307e](https://github.com/cskiller24/querylex/commit/37b307e65413790de47827f2f488531e539dd79d))
* **memory-improve:** add token frequency map and skip high-frequency tokens ([4db4f58](https://github.com/cskiller24/querylex/commit/4db4f58e30503052965ed090bd9beddfab914cb3))
* **memory-improve:** decay entries older than 90 days from search results ([921a4e3](https://github.com/cskiller24/querylex/commit/921a4e36585fe9c96011a4301b6cdb81adac84b1))
* **memory-improve:** lower memory match threshold from 0.86 to 0.60 ([82c7121](https://github.com/cskiller24/querylex/commit/82c7121975809a4a70f8920737b3d705f9ab7a51))
* **quick-260613-21y:** add interactive delete-db/use-db, human-readable list-dbs ([7772555](https://github.com/cskiller24/querylex/commit/777255504965d8d622d79cdd6c665e7de6efbea9))
* **quick-260613-21y:** consolidate encryption to 'encrypt' command, add non-interactive add-db flags ([ff22609](https://github.com/cskiller24/querylex/commit/ff226091e65b15f588c256f2ced97df4f4efa583))
* **state:** add UpdateDatabase and DeleteDatabase to WorkspaceStore ([f19534c](https://github.com/cskiller24/querylex/commit/f19534c0c94f070f4e893558e7fcecfa243a48f0))
* **stats:** add database connectivity check to workspace-stats ([2fe1553](https://github.com/cskiller24/querylex/commit/2fe1553217033b308373361f8330140d93aeb992))
* **task3:** create machinekey.go with cross-platform machine ID derivation ([e8f7735](https://github.com/cskiller24/querylex/commit/e8f77351814af50a058753a4f55afe36e2849aea))


### Bug Fixes

* remove addEmbeddingsWarning dead code and embedding references ([feea744](https://github.com/cskiller24/querylex/commit/feea744341926d9cc2e002ffbc34824ddde27cd0))


### Documentation

* **260608-8e4:** comprehensive CLI documentation ([14b845d](https://github.com/cskiller24/querylex/commit/14b845d7c4caa1b32e4a38ccc401aaa2e8b9caf9))
* **260611-i6z:** pre-dispatch plan for Execute all improvements ([8a19bfa](https://github.com/cskiller24/querylex/commit/8a19bfa1d154943ad5d779958e8e6406daad18a2))
* **quick-260608-8e4:** CLI documentation ([10af9d0](https://github.com/cskiller24/querylex/commit/10af9d03cfd450c7bc06454a9219512ca3ae621e))
* **quick-260611-i6z:** Execute all improvements from new-improvements.md ([91ae159](https://github.com/cskiller24/querylex/commit/91ae159b8d965d48bec3950f686afad55ff3b6c5))
* **state:** record quick task 260607-v40 ([11e6e6f](https://github.com/cskiller24/querylex/commit/11e6e6f2708e16c631c7c6db2e590fa45b14d5bb))
* **state:** record quick task 260611-i6z ([3c58a8b](https://github.com/cskiller24/querylex/commit/3c58a8b21f2c367f1bbcbe3e132cc44327a0772e))
* **state:** record quick task 260613-0ym ([a38647c](https://github.com/cskiller24/querylex/commit/a38647c30d2bb7407bd315a2578e48a532495ddb))


### Miscellaneous Chores

* **1.x:** release 1.2.0 ([55c4646](https://github.com/cskiller24/querylex/commit/55c4646fd55ebaa1784108da2b10725c957b088e))
* **1.x:** release 1.2.0 ([aa09265](https://github.com/cskiller24/querylex/commit/aa092651e337dcf361275d2c8b839884addc6144))
* add .npmignore file and remove unused files field from package.json ([3b26997](https://github.com/cskiller24/querylex/commit/3b269979d90a28a5a18d5ca828f3a3d37eeb9204))
* **hotfix:** Remove querylex completion scripts for Fish, PowerShell, and Zsh ([eaefb7e](https://github.com/cskiller24/querylex/commit/eaefb7ed44a3a381b417ba1291763f8fc2f361a3))
* **hotfix:** Remove querylex completion scripts for Fish, PowerShell, and Zsh ([c6154dd](https://github.com/cskiller24/querylex/commit/c6154ddb2ef68f7740ebb2b88001f016f371ca19))
* npm versioning ([19ec5b8](https://github.com/cskiller24/querylex/commit/19ec5b88a87fb7e1da23882ee2c062a54ab96b12))
* remove .npmignore file and update package.json to include files section ([24adc91](https://github.com/cskiller24/querylex/commit/24adc91a5eb8990da263fa7b1ef7609a00b3bec7))
* **task3:** remove passphrase.go ([bebfd4c](https://github.com/cskiller24/querylex/commit/bebfd4c664a68df7006f244f7c0dfcfaa9b624ed))


### Code Refactoring

* remove installation script and streamline querylex setup process lazy loading ([89040e8](https://github.com/cskiller24/querylex/commit/89040e879399fb17edc811abb7889528f4459d3b))
* **task3:** remove passphrase auto-unlock from preflight and related files ([a7d96f7](https://github.com/cskiller24/querylex/commit/a7d96f70464eed5c816af6571fe5a4bb205d9078))
* **task3:** remove passphrase prompt from run_adddb.go ([1c97a69](https://github.com/cskiller24/querylex/commit/1c97a69e3fa5bdf758f2cf1aa5ef830fbafc1738))
* **task3:** remove passphrase/scrypt from encrypted.go, use machine key ([6d50fd9](https://github.com/cskiller24/querylex/commit/6d50fd9f9bad0fcbd740bf75d49e9325ab5eccf2))


### Tests

* **task3:** remove passphrase-dependent tests from preflight_test.go ([d365609](https://github.com/cskiller24/querylex/commit/d36560924dfd2fdad144a8cd45fc1a9e50406341))
* **task3:** rewrite credential tests for passphrase-less operation ([e965761](https://github.com/cskiller24/querylex/commit/e9657611d15f5e9456fd96d2706fd0a603890dff))


### Continuous Integration

* add automated release pipeline for 1.x branch ([325a342](https://github.com/cskiller24/querylex/commit/325a34292f2cb83ed56e403c4c92953d10f8066a))
* add release-please manifest with current version 1.1 ([28a113b](https://github.com/cskiller24/querylex/commit/28a113b84a6567d6534c3e2b4584769f18494ab3))
* fix manifest version to semver 1.1.0 ([310b1c1](https://github.com/cskiller24/querylex/commit/310b1c13122c5ccd5171360065f49f75205975e1))
* harden release pipeline ([55d518a](https://github.com/cskiller24/querylex/commit/55d518a2c6d474b17a74beeb6792311c294cffba))
* rename config to release-please-config.json (no dot prefix) ([c4f9b92](https://github.com/cskiller24/querylex/commit/c4f9b92ec4d04cfe6c25c4b876f70eed085e0e79))
* simplify CI to single test job on ubuntu-latest ([6d52bc1](https://github.com/cskiller24/querylex/commit/6d52bc1cc4c9f0ed872e8fd5e445f7b97e1a93e3))
* use PAT instead of GITHUB_TOKEN for release-please to trigger downstream workflows ([5f16306](https://github.com/cskiller24/querylex/commit/5f1630668cb87fd768a2ea957a874101e16d9580))

## [1.2.0](https://github.com/cskiller24/querylex/compare/v1.1.0...v1.2.0) (2026-06-13)


### Features

* **260613-0ym:** add generate-encryption and rotate-encryption CLI commands, enhance workspace-stats connectivity ([b13fcda](https://github.com/cskiller24/querylex/commit/b13fcda7b4c002a60eac7e33c2c99e99a62be133))
* **260613-0ym:** replace machine-ID key derivation with stored generated AES-256 key ([62f85cc](https://github.com/cskiller24/querylex/commit/62f85ccc636e0ecf2cfaed846ad3bdffb2e36290))
* add installation script and update binary check in querylex ([5cc740a](https://github.com/cskiller24/querylex/commit/5cc740a01a01560dbe0a0fc009e506b46db7d8f1))
* **memory-improve:** add bigram tokenization to keyword index and search ([79baf69](https://github.com/cskiller24/querylex/commit/79baf69a0967a794bedbd3869160963910e41555))
* **memory-improve:** add FTS5 virtual table and fallback search ([e706caa](https://github.com/cskiller24/querylex/commit/e706caab2ed6934dfe58fb339884676a788e38ed))
* **memory-improve:** add sqlStructureScore with 0.15 weight ([37b307e](https://github.com/cskiller24/querylex/commit/37b307e65413790de47827f2f488531e539dd79d))
* **memory-improve:** add token frequency map and skip high-frequency tokens ([4db4f58](https://github.com/cskiller24/querylex/commit/4db4f58e30503052965ed090bd9beddfab914cb3))
* **memory-improve:** decay entries older than 90 days from search results ([921a4e3](https://github.com/cskiller24/querylex/commit/921a4e36585fe9c96011a4301b6cdb81adac84b1))
* **memory-improve:** lower memory match threshold from 0.86 to 0.60 ([82c7121](https://github.com/cskiller24/querylex/commit/82c7121975809a4a70f8920737b3d705f9ab7a51))
* **quick-260613-21y:** add interactive delete-db/use-db, human-readable list-dbs ([7772555](https://github.com/cskiller24/querylex/commit/777255504965d8d622d79cdd6c665e7de6efbea9))
* **quick-260613-21y:** consolidate encryption to 'encrypt' command, add non-interactive add-db flags ([ff22609](https://github.com/cskiller24/querylex/commit/ff226091e65b15f588c256f2ced97df4f4efa583))
* **state:** add UpdateDatabase and DeleteDatabase to WorkspaceStore ([f19534c](https://github.com/cskiller24/querylex/commit/f19534c0c94f070f4e893558e7fcecfa243a48f0))
* **stats:** add database connectivity check to workspace-stats ([2fe1553](https://github.com/cskiller24/querylex/commit/2fe1553217033b308373361f8330140d93aeb992))
* **task3:** create machinekey.go with cross-platform machine ID derivation ([e8f7735](https://github.com/cskiller24/querylex/commit/e8f77351814af50a058753a4f55afe36e2849aea))


### Bug Fixes

* remove addEmbeddingsWarning dead code and embedding references ([feea744](https://github.com/cskiller24/querylex/commit/feea744341926d9cc2e002ffbc34824ddde27cd0))


### Documentation

* **260608-8e4:** comprehensive CLI documentation ([14b845d](https://github.com/cskiller24/querylex/commit/14b845d7c4caa1b32e4a38ccc401aaa2e8b9caf9))
* **260611-i6z:** pre-dispatch plan for Execute all improvements ([8a19bfa](https://github.com/cskiller24/querylex/commit/8a19bfa1d154943ad5d779958e8e6406daad18a2))
* **quick-260608-8e4:** CLI documentation ([10af9d0](https://github.com/cskiller24/querylex/commit/10af9d03cfd450c7bc06454a9219512ca3ae621e))
* **quick-260611-i6z:** Execute all improvements from new-improvements.md ([91ae159](https://github.com/cskiller24/querylex/commit/91ae159b8d965d48bec3950f686afad55ff3b6c5))
* **state:** record quick task 260607-v40 ([11e6e6f](https://github.com/cskiller24/querylex/commit/11e6e6f2708e16c631c7c6db2e590fa45b14d5bb))
* **state:** record quick task 260611-i6z ([3c58a8b](https://github.com/cskiller24/querylex/commit/3c58a8b21f2c367f1bbcbe3e132cc44327a0772e))
* **state:** record quick task 260613-0ym ([a38647c](https://github.com/cskiller24/querylex/commit/a38647c30d2bb7407bd315a2578e48a532495ddb))


### Miscellaneous Chores

* add .npmignore file and remove unused files field from package.json ([3b26997](https://github.com/cskiller24/querylex/commit/3b269979d90a28a5a18d5ca828f3a3d37eeb9204))
* **hotfix:** Remove querylex completion scripts for Fish, PowerShell, and Zsh ([eaefb7e](https://github.com/cskiller24/querylex/commit/eaefb7ed44a3a381b417ba1291763f8fc2f361a3))
* **hotfix:** Remove querylex completion scripts for Fish, PowerShell, and Zsh ([c6154dd](https://github.com/cskiller24/querylex/commit/c6154ddb2ef68f7740ebb2b88001f016f371ca19))
* npm versioning ([19ec5b8](https://github.com/cskiller24/querylex/commit/19ec5b88a87fb7e1da23882ee2c062a54ab96b12))
* remove .npmignore file and update package.json to include files section ([24adc91](https://github.com/cskiller24/querylex/commit/24adc91a5eb8990da263fa7b1ef7609a00b3bec7))
* **task3:** remove passphrase.go ([bebfd4c](https://github.com/cskiller24/querylex/commit/bebfd4c664a68df7006f244f7c0dfcfaa9b624ed))


### Code Refactoring

* remove installation script and streamline querylex setup process lazy loading ([89040e8](https://github.com/cskiller24/querylex/commit/89040e879399fb17edc811abb7889528f4459d3b))
* **task3:** remove passphrase auto-unlock from preflight and related files ([a7d96f7](https://github.com/cskiller24/querylex/commit/a7d96f70464eed5c816af6571fe5a4bb205d9078))
* **task3:** remove passphrase prompt from run_adddb.go ([1c97a69](https://github.com/cskiller24/querylex/commit/1c97a69e3fa5bdf758f2cf1aa5ef830fbafc1738))
* **task3:** remove passphrase/scrypt from encrypted.go, use machine key ([6d50fd9](https://github.com/cskiller24/querylex/commit/6d50fd9f9bad0fcbd740bf75d49e9325ab5eccf2))


### Tests

* **task3:** remove passphrase-dependent tests from preflight_test.go ([d365609](https://github.com/cskiller24/querylex/commit/d36560924dfd2fdad144a8cd45fc1a9e50406341))
* **task3:** rewrite credential tests for passphrase-less operation ([e965761](https://github.com/cskiller24/querylex/commit/e9657611d15f5e9456fd96d2706fd0a603890dff))


### Continuous Integration

* add automated release pipeline for 1.x branch ([325a342](https://github.com/cskiller24/querylex/commit/325a34292f2cb83ed56e403c4c92953d10f8066a))
* add release-please manifest with current version 1.1 ([28a113b](https://github.com/cskiller24/querylex/commit/28a113b84a6567d6534c3e2b4584769f18494ab3))
* fix manifest version to semver 1.1.0 ([310b1c1](https://github.com/cskiller24/querylex/commit/310b1c13122c5ccd5171360065f49f75205975e1))
* rename config to release-please-config.json (no dot prefix) ([c4f9b92](https://github.com/cskiller24/querylex/commit/c4f9b92ec4d04cfe6c25c4b876f70eed085e0e79))
* simplify CI to single test job on ubuntu-latest ([6d52bc1](https://github.com/cskiller24/querylex/commit/6d52bc1cc4c9f0ed872e8fd5e445f7b97e1a93e3))
