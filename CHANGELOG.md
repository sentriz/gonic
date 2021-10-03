# Changelog

## [0.14.0](https://www.github.com/sentriz/gonic/compare/v0.13.1...v0.14.0) (2021-10-03)


### Features

* **ci:** add debug build workflow ([2780dba](https://www.github.com/sentriz/gonic/commit/2780dba534b673b1a496d44c9fcc3007fd0f2e62))
* **ci:** pin golangci-lint version ([8f7131e](https://www.github.com/sentriz/gonic/commit/8f7131e25b9ea4207cdb9091ccbae26b5118cdac))
* **ci:** test before release please, and only run extra tests on develop and pull request ([cd5771f](https://www.github.com/sentriz/gonic/commit/cd5771f88635b95955c7d2aea72379411142b777))
* **ci:** use GITHUB_TOKEN for release please ([608504b](https://www.github.com/sentriz/gonic/commit/608504bedc88ec02cef34849cb42fb476dd63e1c))
* create cache directory on startup ([f3bc3ae](https://www.github.com/sentriz/gonic/commit/f3bc3ae78990948e75d0b9757c399aad4e5c3b6b)), closes [#127](https://www.github.com/sentriz/gonic/issues/127)
* **encode:** add hi-gain RG and upsampling support ([616b152](https://www.github.com/sentriz/gonic/commit/616b152fede7d56b77b8ea96bc2b86226d690f93))
* **encode:** add mime-type headers to cache handlers ([4109b5b](https://www.github.com/sentriz/gonic/commit/4109b5b66cbb53e9255fcd216195f8e1a773e48d))
* **encode:** use "true" (unconstrained) VBR for Opus profiles ([b9f8ea7](https://www.github.com/sentriz/gonic/commit/b9f8ea704876eb033986e7e586f16c93e2864df2))
* **jukebox:** reduce complexity and update dependencies ([#154](https://www.github.com/sentriz/gonic/issues/154)) ([3938136](https://www.github.com/sentriz/gonic/commit/393813665abb614fa2e2f57cdd575c4dd083b4b5))
* support filter by genre in browse by folder mode ([b56f00e](https://www.github.com/sentriz/gonic/commit/b56f00e9ace62fc3d60b21eef7e638b1ec5007d7))
* support filter by year in browse by folder mode ([6e2d4f7](https://www.github.com/sentriz/gonic/commit/6e2d4f73c53ab908b5933cfbbc1ffc97584e0a08))
* Support WMA files, including those with embedded album art ([#143](https://www.github.com/sentriz/gonic/issues/143)) ([7100b2b](https://www.github.com/sentriz/gonic/commit/7100b2b241ab5c199aaa17b2631b85b065b383e1))


### Bug Fixes

* **build:** add zlib ([ccc0e3c](https://www.github.com/sentriz/gonic/commit/ccc0e3c58d9fb1975bc0bdcf4f87829e9f705b74))
* **ci:** remove deprecated linters ([3382af7](https://www.github.com/sentriz/gonic/commit/3382af72f19eead97b601eee847fd60b6c50ca34))
* **ci:** trim short hash ([6f26974](https://www.github.com/sentriz/gonic/commit/6f269745a5f678b256b4a715ba236a2b847e4de9))
* **docs:** update ubuntu / systemd service instructions ([ef6dd6c](https://www.github.com/sentriz/gonic/commit/ef6dd6c82a638dcd8aa3254839e2f53580a4ef46)), closes [#126](https://www.github.com/sentriz/gonic/issues/126)
* **encode:** Strip EBU R128 gain tags when using forced-RG transcoding ([#145](https://www.github.com/sentriz/gonic/issues/145)) ([5444d40](https://www.github.com/sentriz/gonic/commit/5444d40018c6f8051fc8d03ef46bd66737dfe1f4))
* return early before type switch in ServeStream ([212a133](https://www.github.com/sentriz/gonic/commit/212a13395d288486f9baa57c2da9bef2d9b6324d)), closes [#152](https://www.github.com/sentriz/gonic/issues/152)
* **scanner:** refactor a bit and fix the issue of repeatedly adding and removing tracks 😎 ([93608d0](https://www.github.com/sentriz/gonic/commit/93608d04b49ebfde3020752802fd665ccfe807bb)), closes [#26](https://www.github.com/sentriz/gonic/issues/26) [#63](https://www.github.com/sentriz/gonic/issues/63)
* **scanner:** spawn interval scans in a goroutine ([c0ca6aa](https://www.github.com/sentriz/gonic/commit/c0ca6aaf0337a23b3f1d2a867884afe89fd4a281)), closes [#63](https://www.github.com/sentriz/gonic/issues/63)
* **scanner:** update changed cover files when scanning ([f50817a](https://www.github.com/sentriz/gonic/commit/f50817a3dcdaf752ac4c9a20c078428846dc2bde)), closes [#158](https://www.github.com/sentriz/gonic/issues/158)
* show "gonic" not version in --help ([3cf3bda](https://www.github.com/sentriz/gonic/commit/3cf3bdafd890ea25247f2bf9af14e775d8d1d148))
* trim newlines when rendering flag values ([4637cf7](https://www.github.com/sentriz/gonic/commit/4637cf70c16d9c4ea30c9604ca79704ec0e3f0c4))

### [0.13.1](https://www.github.com/sentriz/gonic/compare/v0.13.0...v0.13.1) (2021-05-08)


### Bug Fixes

* **docker:** bump alpine / go ([1f941b2](https://www.github.com/sentriz/gonic/commit/1f941b2085815d8aa0bf7ad7f3e44efba20295e8))

## [0.13.0](https://www.github.com/sentriz/gonic/compare/v0.12.3...v0.13.0) (2021-05-08)


### ⚠ BREAKING CHANGES

* **subsonic:** don't return gonic version from responses
* bump to go1.16 and embed version

### Features

* bump to go1.16 and embed version ([6f15589](https://www.github.com/sentriz/gonic/commit/6f15589c0889893b7beda85a81d49878401566f0))
* **ci:** arm builds, push multiple registries ([0622672](https://www.github.com/sentriz/gonic/commit/06226724b718883cff9e9150e60e2eeacc2e0a1c))
* **ci:** use ghcr and auto release ([c2c7eb2](https://www.github.com/sentriz/gonic/commit/c2c7eb249f77eebabc910c70357249a3017523ef))
* **subsonic:** don't return gonic version from responses ([58624f0](https://www.github.com/sentriz/gonic/commit/58624f07dc81c36fda79827cc41ac57e89e18b37))


### Bug Fixes

* **ci:** only test on go1.16 ([e9743f0](https://www.github.com/sentriz/gonic/commit/e9743f0cb0e96e9b4b434141e890a0fa16ce3f18))
* don't clutter db close in main ([e6b7691](https://www.github.com/sentriz/gonic/commit/e6b76915daa2bbd6f259f2b019cde9130c62e326))
* trim newline from version ([a565008](https://www.github.com/sentriz/gonic/commit/a5650084d7969a37765d291a6554984e4ae4e2d9))
