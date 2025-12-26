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
