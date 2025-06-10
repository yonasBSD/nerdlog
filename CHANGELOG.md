# Changelog


## [1.11.0](https://github.com/yonasBSD/nerdlog/compare/v1.10.0...v1.11.0) (2025-06-10)


### Features

* Add :conndebug command which shows connection debug info ([9bae829](https://github.com/yonasBSD/nerdlog/commit/9bae82966703ce0eea56d6959db8f0547ba3e29b))
* Add flags for lstreams config and history files ([ea160d8](https://github.com/yonasBSD/nerdlog/commit/ea160d84a64c32e908221e9acc6a8a4d3b72276a))
* Add some basic support for Apache logs ([19a442e](https://github.com/yonasBSD/nerdlog/commit/19a442e5b3ee651c8bf6913dcbd2298d0e7ce5c7))
* Add support for --set command line flag ([ecae528](https://github.com/yonasBSD/nerdlog/commit/ecae528355ae11bc83796f323f561409263c8585))
* Add support for external ssh binary ([2d93f05](https://github.com/yonasBSD/nerdlog/commit/2d93f051b020c0907d6881d1498e85e13b365f5a))
* Add support for shell init commands in logstream config ([c5d756f](https://github.com/yonasBSD/nerdlog/commit/c5d756f4b6b170f99ee0306011d6767809fb4e13))
* Add version info ([1706904](https://github.com/yonasBSD/nerdlog/commit/1706904aa0733a239f0cd2322e0142b8369b1306))
* **CI:** Add version number to the archive names ([78e4e80](https://github.com/yonasBSD/nerdlog/commit/78e4e80359eace22a61f88c24e4d42b9744eefde))
* **CI:** run tests on CI ([17aaf4d](https://github.com/yonasBSD/nerdlog/commit/17aaf4d944fdb85fe637c150f9c6d0021a850142))
* **CI:** run tests on CI FreeBSD ([3a24a77](https://github.com/yonasBSD/nerdlog/commit/3a24a778ddac247af3e36070d09e1b72bd6fcab9))
* **CI:** run tests on CI MacOS ([d1cafc4](https://github.com/yonasBSD/nerdlog/commit/d1cafc42a7e0c1aa0819f76d6689ded151d52a65))
* **CI:** set up release-please with auto binaries building for releases ([2a9d235](https://github.com/yonasBSD/nerdlog/commit/2a9d2353d3050e75d727825c6db86d09229b5d97))
* Debug-print command to filter logs by time range ([aa164b2](https://github.com/yonasBSD/nerdlog/commit/aa164b289217b6be8919ad7ed83460102d005ee5))
* Don't fail if there is a Match directive in ssh config ([3dad67c](https://github.com/yonasBSD/nerdlog/commit/3dad67ceb5e6314ca51a3daf051ba52a62da444f))
* Handle decreased timestamps gracefully ([136890b](https://github.com/yonasBSD/nerdlog/commit/136890b0c7572963aec9225c91a2f31911970179))
* Implement hard refresh using Alt+Ctrl+R or Shift+F5 ([6799aca](https://github.com/yonasBSD/nerdlog/commit/6799aca0844e384e882566938d9cf5f5dc5c98eb))
* Implement proper support for localhost ([82425bf](https://github.com/yonasBSD/nerdlog/commit/82425bfe4b53494e217030d96efe335f6affe5be))
* Implement refresh using Ctrl+R or F5 ([6c95ca5](https://github.com/yonasBSD/nerdlog/commit/6c95ca5716212f5a22f946a31a60726f9769608f))
* Implement ssh authentication via public keys ([435eeab](https://github.com/yonasBSD/nerdlog/commit/435eeab28bd48dbb71c3cd86647e11a32e8dce78))
* Make the histogram cursor and ruler lighter ([b0d6e21](https://github.com/yonasBSD/nerdlog/commit/b0d6e212e263377f8ed1d5afb96877d90f9f903d))
* **minor:** Add a bit more debug info ([d696530](https://github.com/yonasBSD/nerdlog/commit/d69653095f2cf59399a8ee9e18b258119c27b5b3))
* **minor:** Add GOOS and clipboard info to --version output ([256e334](https://github.com/yonasBSD/nerdlog/commit/256e3343711a13dcbe0f80dd66719135b7cd7ea9))
* Resize the connection dialog when needed ([e85811f](https://github.com/yonasBSD/nerdlog/commit/e85811f4e03c756a717b874a8f6a43f8dae3cb13))
* Respect TZ env var if available ([66fb307](https://github.com/yonasBSD/nerdlog/commit/66fb30729ef94b588727f291f2d4664394cdcfe2))
* **UI:** Add a way to copy message to clipboard ([fa44568](https://github.com/yonasBSD/nerdlog/commit/fa44568d7eb3abd9f4a12bd91a542770bb7f05ba))
* **UI:** Implement debug info dialog ([bc3f8e3](https://github.com/yonasBSD/nerdlog/commit/bc3f8e3a041ba341b0c463d32e92b1ba308efd93))


### Bug Fixes

* Avoid panic if nerdlog was built with CGO_ENABLED=0 ([389a193](https://github.com/yonasBSD/nerdlog/commit/389a193836b8e07de71cf7f9bf39fb8441e2871b))
* **CI:** Don't set CGO_ENABLED=0 ([201f621](https://github.com/yonasBSD/nerdlog/commit/201f621f8d7c2472a098051dc4b93a233874f395))
* **CI:** Fix the path to main package ([2ae940b](https://github.com/yonasBSD/nerdlog/commit/2ae940b851bcb56c058c2286bc43b589a684c80d))
* **CI:** Install libx11-dev to the runner ([adde3f5](https://github.com/yonasBSD/nerdlog/commit/adde3f5a12d79ced4ad5c5b8dcdbdff568086147))
* Don't assume that bash is in /bin ([ddcde2b](https://github.com/yonasBSD/nerdlog/commit/ddcde2bdacc2d9d4140f3490ef463d10a8f0ca0e))
* Don't use [[  ]] in main ssh session ([d8b5af0](https://github.com/yonasBSD/nerdlog/commit/d8b5af0eff05d7cf3cfaff7c4d86cb9bc91b0a80))
* Don't use localhost as a default on Windows ([03e7d96](https://github.com/yonasBSD/nerdlog/commit/03e7d96dccef4e685b6c362c38f7d7888b25fc46))
* Fix FreeBSD build with CGO_ENABLED=1 ([db2665f](https://github.com/yonasBSD/nerdlog/commit/db2665fc4ab68a772833cd34a2d016ed0dcaa74d))
* Fix going from May to Jun in traditional syslog format ([de21e4d](https://github.com/yonasBSD/nerdlog/commit/de21e4dc2bc299acda719896106e2cb48920eb4b))
* Fix journalctl pagination with the pattern ([8c85b16](https://github.com/yonasBSD/nerdlog/commit/8c85b16428ea414a687c0bc812071c9567a58483))
* Fix pagination of journalctl-powered logstreams ([f7e373e](https://github.com/yonasBSD/nerdlog/commit/f7e373e0e2c119ddc662593b2af45909319d992f))
* Fix time format for older versions of journalctl ([528974d](https://github.com/yonasBSD/nerdlog/commit/528974d988d633d9b0a000c18b9e05f42d6f8caa))
* Forget queued commands when reconnecting ([04cbe37](https://github.com/yonasBSD/nerdlog/commit/04cbe3725f818ec6617e7888e97bf8bfc7bf644d))
* Handle multiline records from journalctl ([af591a0](https://github.com/yonasBSD/nerdlog/commit/af591a089ee66b0f5d8cba411faaea4dd15c56e3))
* Improve error message when initial query is invalid ([dc9e5fc](https://github.com/yonasBSD/nerdlog/commit/dc9e5fc8e0f5d7e8010743c8e98307cc6642d146))
* Let nerdlog start without ssh-agent ([35ae609](https://github.com/yonasBSD/nerdlog/commit/35ae609f81445094590ac26c3e94633baa53652d))
* Make Makefile compabible with FreeBSD ([d8776b6](https://github.com/yonasBSD/nerdlog/commit/d8776b6238d479a91555f7b887f791e445cc94c1))
* Make sure that the agent script is in home dir ([6885b88](https://github.com/yonasBSD/nerdlog/commit/6885b88b34b34a0f1b4bda3ea3275fbfce271ddb))
* Make the agent script work on FreeBSD ([bcb3ac4](https://github.com/yonasBSD/nerdlog/commit/bcb3ac4206722aa98f80b9e567771c5ba371a878))
* **minor:** Debug-print the skipped-lines info to stderr ([920f57e](https://github.com/yonasBSD/nerdlog/commit/920f57e74cd171cd56c8bc6308a384f5bb81f5d9))
* **minor:** Improve version info generated by make ([564a20f](https://github.com/yonasBSD/nerdlog/commit/564a20fc45fbc7e6e46ccf5496395bb7ab66e01e))
* **minor:** Sort connection errors by host ([1612e91](https://github.com/yonasBSD/nerdlog/commit/1612e913f1dc4126b212a7632664a5b686322fb6))
* Support ancient bash shipped with MacOS ([4c5edd5](https://github.com/yonasBSD/nerdlog/commit/4c5edd5eb5bea8769644d3885f64b8c59a5837d0))
* **UI:** Apply the timezone changes everywhere in UI ([8b523f6](https://github.com/yonasBSD/nerdlog/commit/8b523f6be63c26d736bc4baa956bba5717c8e0fb))
* **UI:** Fix messagebox height if text has empty lines ([ae72eec](https://github.com/yonasBSD/nerdlog/commit/ae72eec086e11a4fa146620d8773d4a7979dd088))
* **UI:** Fix timezone label in query edit form ([e3d7f26](https://github.com/yonasBSD/nerdlog/commit/e3d7f26a2e8b40e206bf1cb021f1a6de1a174f4d))
* **UI:** Make the right edge of the ruler correct in all cases ([c04005b](https://github.com/yonasBSD/nerdlog/commit/c04005b9c70ec7774061e022a346580c0c8312ed))
* Update tview to the same version as in current Debian ([7dce1ff](https://github.com/yonasBSD/nerdlog/commit/7dce1ffdc3d1a724eac15f93d598f30092cd756d))
* Upgrade clipboard package to fix init error handling ([313a4f7](https://github.com/yonasBSD/nerdlog/commit/313a4f7ff25629c88ada36a363297e4b7075c1a3))
* Use /bin/sh as the main ssh session shell ([10e3551](https://github.com/yonasBSD/nerdlog/commit/10e355177a065f9228f14ed0151edcbe1b547bb7))

## [1.10.0](https://github.com/dimonomid/nerdlog/compare/v1.9.0...v1.10.0) (2025-06-09)


### Features

* Add support for external `ssh` binary ([2d93f05](https://github.com/dimonomid/nerdlog/commit/2d93f051b020c0907d6881d1498e85e13b365f5a)), activate it by using `--set 'transport=ssh-bin'`
* Add `:conndebug` command which shows connection debug info ([9bae829](https://github.com/dimonomid/nerdlog/commit/9bae82966703ce0eea56d6959db8f0547ba3e29b))
* Add some basic support for Apache logs ([19a442e](https://github.com/dimonomid/nerdlog/commit/19a442e5b3ee651c8bf6913dcbd2298d0e7ce5c7))

## [1.9.0](https://github.com/dimonomid/nerdlog/compare/v1.8.2...v1.9.0) (2025-06-02)


### Features

* Add support for --set command line flag ([ecae528](https://github.com/dimonomid/nerdlog/commit/ecae528355ae11bc83796f323f561409263c8585))


### Bug Fixes

* Fix going from May to Jun in traditional syslog format ([de21e4d](https://github.com/dimonomid/nerdlog/commit/de21e4dc2bc299acda719896106e2cb48920eb4b))
* Improve error message when initial query is invalid ([dc9e5fc](https://github.com/dimonomid/nerdlog/commit/dc9e5fc8e0f5d7e8010743c8e98307cc6642d146))
* Update tview to the same version as in current Debian ([7dce1ff](https://github.com/dimonomid/nerdlog/commit/7dce1ffdc3d1a724eac15f93d598f30092cd756d))

## [1.8.2](https://github.com/dimonomid/nerdlog/compare/v1.8.1...v1.8.2) (2025-05-25)


### Bug Fixes

* Don't use `[[  ]]` in the main ssh session, which runs in `/bin/sh`. It wasn't a critical issue and things kept working, but an error during bootstrap wouldn't be detected properly ([d8b5af0](https://github.com/dimonomid/nerdlog/commit/d8b5af0eff05d7cf3cfaff7c4d86cb9bc91b0a80))

### Internal or minor changes

* We now run end-to-end tests on the released binaries ([8d9335a](https://github.com/dimonomid/nerdlog/commit/8d9335ae69019c43b7842d7f514a5c80b0bd8434))
* To support the end-to-end tests, some flags were added to override default file locations: `--lstreams-config`, `--cmdhistory-file`, `--queryhistory-file`. These flags might be useful outside of testing as well ([ea160d8](https://github.com/dimonomid/nerdlog/commit/ea160d84a64c32e908221e9acc6a8a4d3b72276a))

## [1.8.1](https://github.com/dimonomid/nerdlog/compare/v1.8.0...v1.8.1) (2025-05-20)


### Bug Fixes

* Use `/bin/sh` as the main ssh session shell ([10e3551](https://github.com/dimonomid/nerdlog/commit/10e355177a065f9228f14ed0151edcbe1b547bb7))
* Forget queued commands when reconnecting, to avoid potential panic ([04cbe37](https://github.com/dimonomid/nerdlog/commit/04cbe3725f818ec6617e7888e97bf8bfc7bf644d))

## [1.8.0](https://github.com/dimonomid/nerdlog/compare/v1.7.2...v1.8.0) (2025-05-19)


### Features

* Respect TZ env var on the remote hosts if available ([66fb307](https://github.com/dimonomid/nerdlog/commit/66fb30729ef94b588727f291f2d4664394cdcfe2))
* Add support for shell init commands in logstream config ([c5d756f](https://github.com/dimonomid/nerdlog/commit/c5d756f4b6b170f99ee0306011d6767809fb4e13))
* **UI:** Add a way to copy message to clipboard ([fa44568](https://github.com/dimonomid/nerdlog/commit/fa44568d7eb3abd9f4a12bd91a542770bb7f05ba))
* **UI:** Implement debug info dialog ([bc3f8e3](https://github.com/dimonomid/nerdlog/commit/bc3f8e3a041ba341b0c463d32e92b1ba308efd93), [aa164b2](https://github.com/dimonomid/nerdlog/commit/aa164b289217b6be8919ad7ed83460102d005ee5), [d696530](https://github.com/dimonomid/nerdlog/commit/d69653095f2cf59399a8ee9e18b258119c27b5b3))

Also, doesn't affect the actual app functionality, but the whole `core` package is now covered with tests (as opposed to just the agent script), and these tests run on CI on Linux, FreeBSD and MacOS.

### Bug Fixes

* Fix journalctl pagination with the pattern ([8c85b16](https://github.com/dimonomid/nerdlog/commit/8c85b16428ea414a687c0bc812071c9567a58483))
* Support ancient bash shipped with MacOS ([4c5edd5](https://github.com/dimonomid/nerdlog/commit/4c5edd5eb5bea8769644d3885f64b8c59a5837d0))
* Upgrade clipboard package to fix init error handling ([313a4f7](https://github.com/dimonomid/nerdlog/commit/313a4f7ff25629c88ada36a363297e4b7075c1a3))
* **UI:** Fix messagebox height if text has empty lines ([ae72eec](https://github.com/dimonomid/nerdlog/commit/ae72eec086e11a4fa146620d8773d4a7979dd088))

## [1.7.2](https://github.com/dimonomid/nerdlog/compare/v1.7.1...v1.7.2) (2025-05-14)


### Bug Fixes

* Fix pagination of journalctl-powered logstreams ([f7e373e](https://github.com/dimonomid/nerdlog/commit/f7e373e0e2c119ddc662593b2af45909319d992f))

## [1.7.1](https://github.com/dimonomid/nerdlog/compare/v1.7.0...v1.7.1) (2025-05-11)


### Bug Fixes

* Fix time format for older versions of journalctl ([528974d](https://github.com/dimonomid/nerdlog/commit/528974d988d633d9b0a000c18b9e05f42d6f8caa))

## [1.7.0](https://github.com/dimonomid/nerdlog/compare/v1.6.0...v1.7.0) (2025-05-11)


### Features

* Handle decreased timestamps gracefully ([136890b](https://github.com/dimonomid/nerdlog/commit/136890b0c7572963aec9225c91a2f31911970179))
* Make the histogram cursor and ruler lighter ([b0d6e21](https://github.com/dimonomid/nerdlog/commit/b0d6e212e263377f8ed1d5afb96877d90f9f903d))
* minor: Add GOOS and clipboard info to `--version` output ([256e334](https://github.com/dimonomid/nerdlog/commit/256e3343711a13dcbe0f80dd66719135b7cd7ea9))


### Bug Fixes

* Make Makefile compabible with FreeBSD ([d8776b6](https://github.com/dimonomid/nerdlog/commit/d8776b6238d479a91555f7b887f791e445cc94c1))
* **UI:** Make the right edge of the ruler correct in all cases ([c04005b](https://github.com/dimonomid/nerdlog/commit/c04005b9c70ec7774061e022a346580c0c8312ed))
* minor: Improve version info generated by make ([564a20f](https://github.com/dimonomid/nerdlog/commit/564a20fc45fbc7e6e46ccf5496395bb7ab66e01e))

## [1.6.0](https://github.com/dimonomid/nerdlog/compare/v1.5.0...v1.6.0) (2025-05-05)


### Features

* Add support for `--version` flag and `:version` / `:about` commands ([1706904](https://github.com/dimonomid/nerdlog/commit/1706904aa0733a239f0cd2322e0142b8369b1306))
* **CI:** Add version number to the archive names ([78e4e80](https://github.com/dimonomid/nerdlog/commit/78e4e80359eace22a61f88c24e4d42b9744eefde))


### Bug Fixes

* Make sure that the agent script runs with `$HOME` as working dir ([6885b88](https://github.com/dimonomid/nerdlog/commit/6885b88b34b34a0f1b4bda3ea3275fbfce271ddb))

## [1.5.0](https://github.com/dimonomid/nerdlog/compare/v1.3.0...v1.5.0) (2025-05-04)


### Features

* Implement ssh authentication via public keys in addition to ssh-agent ([435eeab](https://github.com/dimonomid/nerdlog/commit/435eeab28bd48dbb71c3cd86647e11a32e8dce78))
* Don't fail if there is a `Match` directive in ssh config ([3dad67c](https://github.com/dimonomid/nerdlog/commit/3dad67ceb5e6314ca51a3daf051ba52a62da444f))
* **UI:** Resize the connection dialog when needed ([e85811f](https://github.com/dimonomid/nerdlog/commit/e85811f4e03c756a717b874a8f6a43f8dae3cb13))


### Bug Fixes

* FreeBSD is tested and supported both as a client (where the Nerdlog app runs) and as a remote host (where logs are collected from):
    * Make the agent script work on FreeBSD ([bcb3ac4](https://github.com/dimonomid/nerdlog/commit/bcb3ac4206722aa98f80b9e567771c5ba371a878))
    * Fix FreeBSD build with `CGO_ENABLED=1` ([db2665f](https://github.com/dimonomid/nerdlog/commit/db2665fc4ab68a772833cd34a2d016ed0dcaa74d))
* Don't use localhost as a default on Windows ([03e7d96](https://github.com/dimonomid/nerdlog/commit/03e7d96dccef4e685b6c362c38f7d7888b25fc46))


## [1.3.0](https://github.com/dimonomid/nerdlog/compare/v1.2.4...v1.3.0) (2025-05-03)


### Features

* Implement proper support for localhost ([82425bf](https://github.com/dimonomid/nerdlog/commit/82425bfe4b53494e217030d96efe335f6affe5be))
* Implement refresh using Ctrl+R or F5 ([6c95ca5](https://github.com/dimonomid/nerdlog/commit/6c95ca5716212f5a22f946a31a60726f9769608f))
* Implement hard refresh using Alt+Ctrl+R or Shift+F5 ([6799aca](https://github.com/dimonomid/nerdlog/commit/6799aca0844e384e882566938d9cf5f5dc5c98eb))


### Bug Fixes

* Don't assume that bash is in /bin ([ddcde2b](https://github.com/dimonomid/nerdlog/commit/ddcde2bdacc2d9d4140f3490ef463d10a8f0ca0e))
* Handle multiline records from journalctl ([af591a0](https://github.com/dimonomid/nerdlog/commit/af591a089ee66b0f5d8cba411faaea4dd15c56e3))
* **UI:** Apply the timezone changes everywhere in UI ([8b523f6](https://github.com/dimonomid/nerdlog/commit/8b523f6be63c26d736bc4baa956bba5717c8e0fb))
* **UI:** Fix timezone label in query edit form ([e3d7f26](https://github.com/dimonomid/nerdlog/commit/e3d7f26a2e8b40e206bf1cb021f1a6de1a174f4d))

## [1.2.4](https://github.com/dimonomid/nerdlog/compare/v1.2.3...v1.2.4) (2025-04-28)


### Bug Fixes

* Avoid panic if nerdlog was built with CGO_ENABLED=0 ([389a193](https://github.com/dimonomid/nerdlog/commit/389a193836b8e07de71cf7f9bf39fb8441e2871b))

## [1.2.3](https://github.com/dimonomid/nerdlog/compare/v1.2.2...v1.2.3) (2025-04-28)


### Bug Fixes

* **CI:** Install libx11-dev to the runner ([adde3f5](https://github.com/dimonomid/nerdlog/commit/adde3f5a12d79ced4ad5c5b8dcdbdff568086147))

## [1.2.2](https://github.com/dimonomid/nerdlog/compare/v1.2.1...v1.2.2) (2025-04-28)


### Bug Fixes

* **CI:** Don't set CGO_ENABLED=0 ([201f621](https://github.com/dimonomid/nerdlog/commit/201f621f8d7c2472a098051dc4b93a233874f395))

## [1.2.1](https://github.com/dimonomid/nerdlog/compare/v1.2.0...v1.2.1) (2025-04-28)


### Bug Fixes

* **CI:** Fix the path to main package ([2ae940b](https://github.com/dimonomid/nerdlog/commit/2ae940b851bcb56c058c2286bc43b589a684c80d))

## [1.2.0](https://github.com/dimonomid/nerdlog/compare/v1.1.0...v1.2.0) (2025-04-28)


### Features

* Add support for `journalctl` ([6d7d69](https://github.com/dimonomid/nerdlog/commit/6d7d695ced450e1648994e690dd26a503b4fe034), [1687ee](https://github.com/dimonomid/nerdlog/commit/1687ee728387d838c9ec56d40b3b2a3d9acf7901)). Log files are still preferred, because [they are more reliable](https://github.com/dimonomid/nerdlog/issues/7#issuecomment-2820521885) and [work much faster](https://github.com/dimonomid/nerdlog/issues/7#issuecomment-2823303380), but `journalctl` is more universally available these days, and also often has longer logs history, so it is fully supported now.
* Add support for reading logs with `sudo` ([23a6a4](https://github.com/dimonomid/nerdlog/commit/23a6a4e6b48da8658fcfd0eefb0b2193ba389a13))
* Add `--ssh-config` flag to specify the ssh config location ([3ceb70](https://github.com/dimonomid/nerdlog/commit/3ceb70b803bff5b47e3982b8dd202516d2bbd538))
* **CI:** set up release-please with auto binaries building for releases ([2a9d23](https://github.com/dimonomid/nerdlog/commit/2a9d2353d3050e75d727825c6db86d09229b5d97))

### Bug Fixes

* Fix focus issue when non-last modal is removed ([2e4ff3](https://github.com/dimonomid/nerdlog/commit/2e4ff3d35f7e473283b7afe7671d6e0e180d2dac))

## 1.1.0 (2025-04-26)

### Features

* Support keyboard shortcuts `Alt+Left` and `Alt+Right` for navigating
  browser-like history, just as it works in a browser;
* Rename the binary installed with `go install
  github.com/dimonomid/nerdlog/cmd/nerdlog@latest` from `nerdlog-tui` to just
  `nerdlog`.

## 1.0.0 (2025-04-22)

### Features

* First release which can be considered a minimal viable product.

  There’s still plenty of room for new features, and some minor bug fixes, but
  overall the core functionality is in place and stable enough to be released
  into the wild.
