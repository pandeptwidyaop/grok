## [1.0.0-alpha.13](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.12...v1.0.0-alpha.13) (2025-12-27)

### Features

* add WebSocket support and reduce minimum subdomain length ([7b01868](https://github.com/pandeptwidyaop/grok/commit/7b0186884e1a84747cdba91000a890b25dc1883e))

### Bug Fixes

* login page reload timing and super admin delete permission ([84f907a](https://github.com/pandeptwidyaop/grok/commit/84f907a62a53a8ded62aec504d70013256e98ae0))

## [1.0.0-alpha.12](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.11...v1.0.0-alpha.12) (2025-12-27)

### Features

* **ui:** add comprehensive mobile optimization and critical fixes ([7cc8bd8](https://github.com/pandeptwidyaop/grok/commit/7cc8bd862dd7a5c04d575eb86926b9d58601f2d4))

### Bug Fixes

* webhook URL parsing and subdomain validation improvements ([ac4ef11](https://github.com/pandeptwidyaop/grok/commit/ac4ef1158d6d3d4e955cc8b45e5ad63692bbf786))

## [1.0.0-alpha.11](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.10...v1.0.0-alpha.11) (2025-12-27)

### Bug Fixes

* **lint:** fix golangci-lint issues in TLS tests ([298d9e4](https://github.com/pandeptwidyaop/grok/commit/298d9e49d4dca5dc71dff95ad7fc69d218ea23b3))

## [1.0.0-alpha.10](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.9...v1.0.0-alpha.10) (2025-12-27)

### Features

* Add Docker release with semantic versioning and attestation ([ad0d598](https://github.com/pandeptwidyaop/grok/commit/ad0d598393f2740111e54b7974fca885c0e4f179))
* **cli:** add TLS flags to set-server command for quick setup ([7463471](https://github.com/pandeptwidyaop/grok/commit/7463471442091ed2f24d11c93ed41b4ae0874158))
* **dashboard:** add release channel selector (stable/beta/alpha) ([672c8fd](https://github.com/pandeptwidyaop/grok/commit/672c8fdd32bc53e833825220a7dd7ad338110ef1))
* **dashboard:** add TLS detection and --tls flag to Getting Started ([2083f6a](https://github.com/pandeptwidyaop/grok/commit/2083f6a319c5c68d0892ad4c0f3406213269b5c4)), closes [#10b981](https://github.com/pandeptwidyaop/grok/issues/10b981) [#667](https://github.com/pandeptwidyaop/grok/issues/667)
* Integrate Docker build into release workflow ([ff5f9bf](https://github.com/pandeptwidyaop/grok/commit/ff5f9bf6fbfb1ac52f3ef9ac2a3b3861649122e0))

### Bug Fixes

* Add attestations:write permission to release workflow ([e5708a5](https://github.com/pandeptwidyaop/grok/commit/e5708a5f6763d93eb92292ac5a3823e07ea701d9))

## [1.0.0-alpha.9](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.8...v1.0.0-alpha.9) (2025-12-27)

### Features

* add macOS and Windows ARM64 builds for grok-server ([091bc00](https://github.com/pandeptwidyaop/grok/commit/091bc0050a21d9942d7055298be4f55ceb60f0ec))
* enable TLS for Dashboard API server when certificates are configured ([d54c503](https://github.com/pandeptwidyaop/grok/commit/d54c5036b3a4164041e4c630f4e2877a55b1fd5f))

### Documentation

* **config:** improve JWT secret documentation and generation ([ab1d51d](https://github.com/pandeptwidyaop/grok/commit/ab1d51dd92925b7cdf55d1aa08e76d474ca3475f))

## [1.0.0-alpha.8](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.7...v1.0.0-alpha.8) (2025-12-26)

### Features

* **security:** implement httpOnly cookies and CSRF protection for frontend ([7be3c7c](https://github.com/pandeptwidyaop/grok/commit/7be3c7c2737e90166cb7f0c3220c3138fe889102))

### Code Refactoring

* reduce cyclomatic complexity across codebase ([eee1502](https://github.com/pandeptwidyaop/grok/commit/eee150213763213aaaa00db3b28d5f9ace8bc32b))

## [1.0.0-alpha.7](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.6...v1.0.0-alpha.7) (2025-12-26)

### Bug Fixes

* **test:** fix integration test failures ([a00813b](https://github.com/pandeptwidyaop/grok/commit/a00813bd5543493e3805ec8edc365e41e8ff05cf))
* **webhook:** correct ExtractWebhookComponents to match plan format ([4669a3f](https://github.com/pandeptwidyaop/grok/commit/4669a3f650591751d300886b90b0c229e5857576))

### Code Refactoring

* fix linting issues and improve code quality ([d035b89](https://github.com/pandeptwidyaop/grok/commit/d035b896ceecafe53ccf47313adbdc9a460881e4))

## [1.0.0-alpha.6](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.5...v1.0.0-alpha.6) (2025-12-26)

### Bug Fixes

* **ci:** generate proto files before linting ([a516510](https://github.com/pandeptwidyaop/grok/commit/a516510109881f4949f92ffb36f90b2c5030d30f))

## [1.0.0-alpha.5](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.4...v1.0.0-alpha.5) (2025-12-26)

### Bug Fixes

* Add error checking for type assertions and function calls ([781fb01](https://github.com/pandeptwidyaop/grok/commit/781fb0135d0801115620858e6fa5a5f46dc941e8))
* Add error checking in tunnel_service.go ([884cd2c](https://github.com/pandeptwidyaop/grok/commit/884cd2c249e6b9df6b3b30349f21fd7ec9d03410))
* **build:** copy frontend dist to embed location ([18d242c](https://github.com/pandeptwidyaop/grok/commit/18d242c7c5d6c6abffa7a39b8ec0127d289a328f))
* **build:** remove unnecessary dist copy steps ([aca0efc](https://github.com/pandeptwidyaop/grok/commit/aca0efc6d8dd9c01ac034da964c9aace015e19cd))
* **ci:** add build output verification ([54fd96c](https://github.com/pandeptwidyaop/grok/commit/54fd96c75a8ad3263a1de59860e3b8a32e76417e))
* **ci:** build frontend before linting ([1c0bc8e](https://github.com/pandeptwidyaop/grok/commit/1c0bc8edba008eaf0fc8eef282cc6c17a0c5de00))
* **ci:** ensure parent directory exists before copy ([9a233be](https://github.com/pandeptwidyaop/grok/commit/9a233be1f66cab967f2035aaf75ea312f2a04316))
* **ci:** install golangci-lint from source for Go 1.25 support ([0bed9a3](https://github.com/pandeptwidyaop/grok/commit/0bed9a3144fb016f1c6408fc89af90daafbcc92a))
* **ci:** update Go version to 1.25 and fix golangci-lint config ([2ba50de](https://github.com/pandeptwidyaop/grok/commit/2ba50de7e022e7c7243d5052207bd6962f882f69))
* Complete errcheck linting fixes ([8235f43](https://github.com/pandeptwidyaop/grok/commit/8235f43a71803691cb976a078103f43a4d931a0d))
* resolve all critical linter warnings ([c642c90](https://github.com/pandeptwidyaop/grok/commit/c642c90a5ec4f3450ee4ed8a18c3fecabe7a2ef0))
* Update golangci-lint config for v1.64.8 ([53799be](https://github.com/pandeptwidyaop/grok/commit/53799beab49586ce22aaae038ceaf8acbe5f7a3a))

## [1.0.0-alpha.4](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.3...v1.0.0-alpha.4) (2025-12-26)

### Bug Fixes

* **ci:** remove unnecessary copy step in test workflow ([b70deb8](https://github.com/pandeptwidyaop/grok/commit/b70deb865c08fc0b9e754fa3b7e1172d03288a9c))

## [1.0.0-alpha.3](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.2...v1.0.0-alpha.3) (2025-12-26)

### Bug Fixes

* **ci:** build web dashboard before running tests ([be873ca](https://github.com/pandeptwidyaop/grok/commit/be873cadd51c0f9f288fdbdcdf3e2974ded6eb6c))
* **ci:** enable CGO for server builds to support SQLite ([947f4b5](https://github.com/pandeptwidyaop/grok/commit/947f4b5c1ce3bd81c1fa76609f2c646aa20c8e9f))
* **ci:** remove ARM architecture builds to avoid CGO cross-compilation issues ([8aefb38](https://github.com/pandeptwidyaop/grok/commit/8aefb38f4a0a7a217c616f45d416b131346bef38))
* **docker:** enable CGO for SQLite support ([c9551d2](https://github.com/pandeptwidyaop/grok/commit/c9551d2a408e7deac18fec639bd2bf4f41d4c468))
* use HTTPS for public URLs when TLS is enabled and remove redundant test jobs ([c277864](https://github.com/pandeptwidyaop/grok/commit/c2778640b3e56f8fdc0c406fbd59b4b6cc765553))

### Code Refactoring

* **ci:** simplify test workflow ([335ce46](https://github.com/pandeptwidyaop/grok/commit/335ce46315c74543452f9576ba0d633add56d188))

## [1.0.0-alpha.2](https://github.com/pandeptwidyaop/grok/compare/v1.0.0-alpha.1...v1.0.0-alpha.2) (2025-12-26)

### Bug Fixes

* **ci:** add proto generation to all test workflows ([b4bbbc0](https://github.com/pandeptwidyaop/grok/commit/b4bbbc039abb3bfa9d15db5fae36a4871bd6d68c))
* **docker:** include scripts directory in build context ([1d1ee4e](https://github.com/pandeptwidyaop/grok/commit/1d1ee4ee06a26e33e8aaefe95d7c72d8f452b9d7))

## 1.0.0-alpha.1 (2025-12-26)

### Features

* Add byte tracking, auto-reconnection, and stale tunnel cleanup ([98fa458](https://github.com/pandeptwidyaop/grok/commit/98fa45817c92d0d916eab70c2d49e25e57d211b3))
* Add complete Two-Factor Authentication (2FA) system ([110258c](https://github.com/pandeptwidyaop/grok/commit/110258cf0e11373c998a9b6bdc5b613da34fb260))
* add comprehensive Getting Started tutorial and update CI/CD for beta releases ([ec31e57](https://github.com/pandeptwidyaop/grok/commit/ec31e5724dfa0c650239d2f9e2a0f6ac5e67c994))
* Add comprehensive testing and fix critical bugs ([9c6e9ca](https://github.com/pandeptwidyaop/grok/commit/9c6e9ca57502412035bfd9d8f1575163aa593f30))
* Add comprehensive TLS support with gencert command ([a326183](https://github.com/pandeptwidyaop/grok/commit/a326183df93022bf3c3cc5966c2ad337ed778eaf))
* Add custom Grok SVG icon throughout the application ([eeedf80](https://github.com/pandeptwidyaop/grok/commit/eeedf8057f952251f17b086b0a434b1178dd5ec2)), closes [#667](https://github.com/pandeptwidyaop/grok/issues/667)
* add Docker support with multi-arch builds and GHCR publishing ([a631e3b](https://github.com/pandeptwidyaop/grok/commit/a631e3b726a09daa066c3b6882eb6b4e47d91bb6))
* Add E2E verification and comprehensive testing summary ([2006539](https://github.com/pandeptwidyaop/grok/commit/20065396dc89f53f33ae011288361cdbc15f4e52))
* add health check endpoint for Docker health monitoring ([36b123d](https://github.com/pandeptwidyaop/grok/commit/36b123dcf8062c0ed25640d942e71e631bcc3fc9))
* Add multi-tenancy, webhook system, and real-time stats ([5340547](https://github.com/pandeptwidyaop/grok/commit/5340547242cf7787bf6ba19ade1033a6c8001073))
* Add Organization and Owner columns to Tunnels table ([b106eb5](https://github.com/pandeptwidyaop/grok/commit/b106eb5bf5231987869a9d844510bffff6ed75a2))
* Add Organization and User columns to webhook apps table ([710d598](https://github.com/pandeptwidyaop/grok/commit/710d5988c78548e527d79e4083745f194518da44))
* Add self-update command for client and server ([5863f64](https://github.com/pandeptwidyaop/grok/commit/5863f64a5436970331b2152004b2c0a32c49434f))
* Add set-server config command ([2ff7ff6](https://github.com/pandeptwidyaop/grok/commit/2ff7ff6ffc5560eec02a35bf1a5e25328b680cc9))
* Add TLS configuration commands ([6bbfe7e](https://github.com/pandeptwidyaop/grok/commit/6bbfe7e761da79a9e02ad9c76cda801f9a6316ab))
* Add Type column to Tunnels table ([b784cf8](https://github.com/pandeptwidyaop/grok/commit/b784cf85020eaffcd1732b8daedf223bc623d36b))
* Change Status chips to outlined variant in Auth Tokens ([eef91f2](https://github.com/pandeptwidyaop/grok/commit/eef91f23b1850786e0a9d064cb5b52a54792811f))
* Convert Organization list from grid to table layout ([e9559f6](https://github.com/pandeptwidyaop/grok/commit/e9559f6c2338026a1b64485d6a1aba3720349b30))
* Create Organization Detail page and move actions from list ([fa4ce96](https://github.com/pandeptwidyaop/grok/commit/fa4ce96b3a8ed5db0dd2a6efaabe083095eee93b))
* Development workflow with hot reload and auth persistence fix ([f47e552](https://github.com/pandeptwidyaop/grok/commit/f47e5521c23da151d7eaed871b4a6cc69e95ee9c))
* Display organization name as chip in webhook apps table ([6b31aa7](https://github.com/pandeptwidyaop/grok/commit/6b31aa7cd2b0354bb653cd56a81fba0c2269810f))
* Filter webhook routes to HTTP tunnels only ([a7deb4e](https://github.com/pandeptwidyaop/grok/commit/a7deb4e2850a036ac949422a87a9979730c0a7e3))
* implement complete ngrok clone with dashboard ([ad776ea](https://github.com/pandeptwidyaop/grok/commit/ad776ea2f0db3a7970ff209752fcfe2baf671a96))
* Implement webhook broadcast routing with tunnel proxying ([589f66d](https://github.com/pandeptwidyaop/grok/commit/589f66ddf7d7238752720b8f1e21337829c5ba46))
* Redesign web panel with professional blue tech theme ([ab10517](https://github.com/pandeptwidyaop/grok/commit/ab10517c08d280a41af9af8cb81d2abc9010c619))

### Bug Fixes

* Add JSON tags to Tunnel model for snake_case API responses ([5d83915](https://github.com/pandeptwidyaop/grok/commit/5d839152c9d9cda45e058a08fe24801f94943ccb))
* **ci:** build frontend before server compilation and package releases as tar.gz archives ([732c42f](https://github.com/pandeptwidyaop/grok/commit/732c42f3ec8c5ff40c5de5468493ef67e21721ff))
* **ci:** correct tar archive creation syntax ([95c111b](https://github.com/pandeptwidyaop/grok/commit/95c111b35f096a8f064cc544b5585bb13254ef63))
* Hide pending-allocation subdomain for TCP tunnels ([406d2ae](https://github.com/pandeptwidyaop/grok/commit/406d2ae40b287642cdabaa185c8703de7dba6cae))
* JWT token persistence after page refresh ([0c194a9](https://github.com/pandeptwidyaop/grok/commit/0c194a9859926a652ae33a20fcd00b5056e8f10e))
* Subdomain extraction bug causing slash prefix ([00f4387](https://github.com/pandeptwidyaop/grok/commit/00f43877a3eae6195ee22db3e2e465484a371419))
* update Docker ports to match server configuration ([767ef52](https://github.com/pandeptwidyaop/grok/commit/767ef523b30d4f3047b45b34fd6d881cb41051e5))
* update Dockerfile to use Go with auto toolchain ([3cf47ae](https://github.com/pandeptwidyaop/grok/commit/3cf47aeb1e6bc496415c40471ea3a88937c88846))
* **web:** remove Tailwind CSS imports and directives ([526de4b](https://github.com/pandeptwidyaop/grok/commit/526de4b933a74d93c137980f2fe8bf4d4fb6b87e))

### Performance Improvements

* implement major tunnel performance optimizations ([8d9eead](https://github.com/pandeptwidyaop/grok/commit/8d9eead3f6673cf48060259ebaa19388e2c24468))

### Documentation

* Add comprehensive testing guide and documentation ([a4b9527](https://github.com/pandeptwidyaop/grok/commit/a4b95278b4a445e5a1c379566a76355c9437b8b6))
* Prepare repository for public release ([95a995c](https://github.com/pandeptwidyaop/grok/commit/95a995c110fd787c74fb53ddd876f2b49b4f5111))
