## [Unreleased](https://github.com/micromdm/micromdm/compare/v1.12.1...main)

- Remove legacy code (#998)
- Fix verifying SHA-2 signatures on on ABM enrollment (#989)
- Fix nil panic in inmem queue (#984)
- Project dependency updates (#976, #981)

## [v1.12.1](https://github.com/micromdm/micromdm/compare/v1.12.0...v1.12.1) March 14, 2024

This release includes one fix.

Thanks to our contributors: @iann0036

- Fix error comparison in builtin stores (#975)

Note: this release was made because v1.12.0 was tagged on a commit not on the main branch. Please use this release instead.

## [v1.12.0](https://github.com/micromdm/micromdm/compare/v1.11.0...v1.12.0) February 17, 2024

This release includes new features and fixes.

Thanks to our contributors: @grahamgilbert, @jamesez, @jessepeterson, @korylprince

- Add `-log-time` flag to include timestamps in log messages (#890)
- Add `-device-signature-skew` flag to allow configuring clock skew when verifying device signatures (#887)
- Tidy code for Go 1.20 and update Go version for Docker and CI (#902)
- Add support for inspecting the MDM command queue (#895)
  - See the [docs](https://github.com/micromdm/micromdm/blob/main/docs/user-guide/api-and-webhooks.md#inspecting-the-command-queue) for how to use!
- Fix HTTP status codes being swallowed by -http-debug flag (#906)
- Remove unused tools package
- Fix pkg signature checks on non-macOS platforms (#930, #962)
- Add go:generate for command_queued protobuf
- Switch to [smallstep/pkcs7](https://github.com/smallstep/pkcs7) for PKCS7 library (#944)
- Remove PayloadScope=System from enrollment profile (See #766)
- Allow disabling default HTTP to HTTPS redirect (#967)
- Project dependency updates (#881, #888, #899, #900, #918, #933, #946, #948, #954, #958, #960, #961, #965, #966)

## [v1.11.0](https://github.com/micromdm/micromdm/compare/v1.10.1...v1.11.0) May 10, 2023

This release includes new features and fixes.

Thanks to our contributors: @williamtheaker, @korylprince, @krmzbbr

- Add Dependabot updating (#857)
- Add SoftwareUpdateSettings to Settings command (#771, #856)
- Add tools script [install_vpp_application](https://github.com/micromdm/micromdm/blob/main/tools/api/commands/install_vpp_application) to install VPP apps (#865)
  - Note MicroMDM doesn't support VPP/A&B itself.
- Fix bug that prevented errors being logged on the Checkin and Connect endpoints (#871)
- Add support for submitting "raw" plist MDM commands! (#864)
  - See the [docs](https://github.com/micromdm/micromdm/blob/main/docs/user-guide/api-and-webhooks.md#schedule-raw-commands-with-the-api) for how to use!
- Add [NanoMDM](https://github.com/micromdm/nanomdm)-compatible Declarative Device Management (DDM) "proxy" support. (#882)
  - Use the `-dm` switch to extract and forward the Declarative Management protocol Endpoints to a specialized HTTP server. Such as [KMFDDM](https://github.com/jessepeterson/kmfddm).
  - Check out the older [blog post about DDM](https://micromdm.io/blog/wwdc21-declarative-management/) for more info.
- Project dependency updates (#858, #859, #860, #861, #867, #869, #873, #872, #874, #875, #879)

## [v1.10.1](https://github.com/micromdm/micromdm/compare/v1.10.0...v1.10.1) January 24, 2023

Patch release. Thanks to our contributors: @korylprince

- Fix certificate check for DEP and OTA enrollment attempts (#854, #855)

## [v1.10.0](https://github.com/micromdm/micromdm/compare/v1.9.0...v1.10.0) January 13, 2023

This release includes new features and fixes.

Thanks to our contributors for this release: @networkpanic, @meta-github, @korylprince, @ayush5harma, @discentem, @hrgbcxd, @petitout

- Add support for `Priority` in ScheduleOSUpdate command (#803)
- Added RefreshCellularPlans support (#809)
- Add LOM commands (#839)
  - See Apple's newer [documentation on configuring MDM via LOM](https://support.apple.com/guide/deployment/lights-out-management-payload-settings-dep580cf25bc/web)
  - Including [example configurations](https://support.apple.com/guide/deployment/lom-configuration-command-issuance-dep5e14207f2/1/web/1.0)
- Fix RotateFileVaultKey panics (#801)
- Fix for DEP client library to be thread safe (#850)
- Add ability to sign apps when they're uploaded: `mdmctl apply app -sign-identity` (#785)
- `mdmctl`: don't send a request body for GET requests (#821)
- Updated `schedule_os_update` script to support more parameters. (#828)
- Switch to using [cfgprofiles](https://github.com/jessepeterson/cfgprofiles) for enrollment profile generation (#827)
- Documentation & CLI usage improvements (#799, #811, #817, #819, #823)

## [v1.9.0](https://github.com/micromdm/micromdm/compare/v1.8.0...v1.9.0) January 27, 2022

- Add new fields for the ScheduleOSUpdate command (#793)
- Use HTTPS for retrieving Apple certificates (#792)
- Add CLI help for `mdmctl config switch` (#791)
- Update builder to Go 1.17 (#783)
- Replace gogo/protobuf with Google protobuf (#773)
- Add GHCR container workflow. Add ARM build. (#745)
- New in-memory-only command queue (#736)
- Bootstrap token support (#781, #782)
- Fix potential DEP sync data loss (#779)
- Support logging proxy IP headers `X-Forwarded-For`, `X-Real-IP`. Enable with `-http-proxy-headers` switch (#744)
- Allow MDM check-in messages to return data (#764)
- Added support for SetRecoveryLock and VerifyRecoveryLock (#757)
- Fix SetFirmwarePassword and VerifyFirmwarePassword parameters (#743)
- Command UUID can now be passed in as as a request parameter (#754)
- Update to SCEP v2, switch to Mozilla PKCS7, interface cleanup (#737, #772, #778)
- Fix panic when using DEP mdmctl commands with no DEP tokens configured (#750)
- Spruce up built-in landing page including better accessibility (#721, #751)
- Documentation & CLI usage improvements (#729, #730, #748, #749)

Thanks to our contributors for this release: @discentem, @korylprince, @williamtheaker, @bpmcneilly, @daemonsy, @tomaswallentinus, @ivanhata, @networkpanic, @HernanPaez

## [v1.9.0-beta](https://github.com/micromdm/micromdm/compare/v1.8.0...v1.9.0-beta) October, 2021

- Replace gogo/protobuf with Google protobuf (#773)
- Add GHCR container workflow. Add ARM build. (#745)
- New in-memory-only command queue (#736)
- Bootstrap token support (#781, #782)
- Fix potential DEP sync data loss (#779)
- Support logging proxy IP headers `X-Forwarded-For`, `X-Real-IP`. Enable with `-http-proxy-headers` switch (#744)
- Allow MDM check-in messages to return data (#764)
- Added support for SetRecoveryLock and VerifyRecoveryLock (#757)
- Fix SetFirmwarePassword and VerifyFirmwarePassword parameters (#743)
- Command UUID can now be passed in as as a request parameter (#754)
- Update to SCEP v2, switch to Mozilla PKCS7, interface cleanup (#737, #772, #778)
- Fix panic when using DEP mdmctl commands with no DEP tokens configured (#750)
- Spruce up built-in landing page including better accessibility (#721, #751)
- Documentation & CLI usage improvements (#729, #730, #748, #749)

Thanks to our contributors for this release: @discentem, @korylprince, @williamtheaker, @bpmcneilly, @daemonsy, @ivanhata

## [v1.8.0](https://github.com/micromdm/micromdm/compare/v1.7.1...v1.8.0) February, 2021

- Fix embedded manifest of InstallEnterpriseApplication (#669)
- Added Activation Lock Bypass support code (#677)
- Fix DEP device serialization so that `ProfileStatus` of device now works (#682)
- mdmctl can now have a base server URL (#683)
- Fix an asymptomatic queue marshaling bug (#690)
- Add ability to unassign DEP devices via API (#687)
- A device's command queue is now cleared during enrollment (#692)
- APNS is now proxy aware (#698)
- Add `-validate-scep-issuer` and `-validate-scep-expiration` flags to only validate the SCEP certificate was issued by the MicrMDM SCEP CA, and optionally to validate that the certificate hasn't expired (#700)
- Add `-udid-cert-auth-warn-only` flag that disables the UDID-certificate authentication mechanism. Can be used to help remediate [expiring device identity certificates](https://github.com/micromdm/micromdm/wiki/Device-Identity-Certificate-Expiration) (#643)
- Fix for multiple InstallApplications in Blueprints (#549, #704)
- More secure argument passing in API scripts (#709)
- TimeZone setting support in Settings command (#719)
- Support tls-alpn-01 for Let's Encrypt certificates (#720)
- Update MDM Vendor CSR signing to SHA-2 and use new Apple intermediate cert (#723, #725)
- Avoid unnecessary command queue save/disk write (#711)
- Documentation updates

Thanks to our contributors for this release: @MobileDan, @meta-gitub, @grahamgilbert, @tperfitt, @williamtheaker, @slawoslawo, @choehn-signogy, @natewalck, @korylprince

## [v1.7.1](https://github.com/micromdm/micromdm/compare/v1.7.0-alpha...v1.7.1) April, 2020

- Replace un-maintained UUID dependency (#665)
- Correctly handle DEP profile removal response (#666)
- Fix permissions on API command tools (#667)
- Fix TLS startup issue (#673)
- Documentation updates

Thanks to our contributors for this release: @bdemetris, @tricknotes, @netproteus

## [v1.7.0-alpha](https://github.com/micromdm/micromdm/compare/v1.6.0...v1.7.0-alpha) March, 2020

### Reliability, scalability, security, and usability improvements:

- Add device DEP status to API response (#617)
- CLI improvements (#618, #620, #621)
- Support new values for AccountConfiguration (#627)
- Fix issue where DEP watcher would stop permanently for transient network issues (#582, #632)
- Workaround issue where a newly added DEP token would not be used after a restart (#546, #633)
- Fix bug with applying an empty blueprint (#615, #634)
- Add `-no-command-history` flag to disable saving of command history (#640). This works around a race-condition/scalability issue with device records (#556).
- Add dynamic SCEP challenges (#642). Require dynamic SCEP challenges for certificate issuance with `-use-dynamic-challenge` and (only recommended for testing) generate them in enrollment profiles with `-gen-dynamic-challenge`.
- Add MDM commands to enable and disable remote desktop (#651)
- SCEP payload key names were corrected (#652)

Thanks to our contributors for this release: @grahamgilbert, @n8felton, @tomaswallentinus

## [v1.6.0](https://github.com/micromdm/micromdm/compare/v1.5.0...v1.6.0) August 14, 2019

### Go security update along with updates:

- Add `erase_device` tools script
- Add assign profile endpoint (#611)
- Add support for User Enrollment (#597)
- Add support for Signing Profiles (#602)
- Add support for setting APNS message expiration (#609)
- Update `mdmctl remove devices -serial` flag to be plural (now `-serials`) (#621)

Thanks to our contributors for this release: @WardsParadox, @n8felton

## [v1.5.0](https://github.com/micromdm/micromdm/compare/v1.4.0...v1.5.0) June 15 2019

- Fix DEP token update issue (#513, #510)
- Refactor certificate verification and implement UDID-cert authentication (#358, #429)
- Cleanup DEP library and integrate into main project (#504, #505)
- Add API endpoint to retrieve APNS certificate (#503)
- Remove deprecated `-apns` flags from server startup (#528)
- Move API calls to list endpoints from HTTP GET to HTTP POST (#522, #523, #524, #525, #526)
- Add support for the ApplicationConfiguration Setting (#521)
- Add support for the ActivationLockBypassCode Command (#578)
- Allow SCEP client validity to be adjusted via server startup flag (#577)
- Fix bug in mdmctl server saving, switch config when saving automatically (#565, #566)
- Do not send DeviceConfigured automatically when there are no blueprints (#586)
- Set acknowledge time when moving command to completed queue (#581)
- Serialize PurchaseMethod when value is 0. (#592)

Thanks to our contributors for this release: @discentem, @nkllkc, @arubdesu, @bdemetris, @Lepidopteron, @joncrain, @emman27, @jenjac, @daniellockard, and @0xflotus

## [v1.4.0](https://github.com/micromdm/micromdm/compare/v1.3.1...v1.4.0) September 6 2018

### Stability Improvements

- Handle DEP INVALID_CURSOR response (#497)
- Use config for block push (#479, #480)
- No longer store SCEP CA on disk or include in enrollment profile (#490)
- Further SCEP fixes (#492, #493)
- Base64 fixes for API CLI tools (#477)
- `mdmctl apply block` now works with self-signed certs (#480)
- Add API CLI tool for dep sync (#481)
- DeviceInformation command API example support query strings (#469)
- Allow setting curl options in environment variable (#455)
- Fix URL params decoding. (#467)
- Reorganize/refactor server init (#458)
- Allow supplying additional `curl` options in API CLI tools (#455)

Thanks to our contributors for this release: @erikng, @gerardkok, @knightsc, @marpaia, and @ochimo!

## [v1.3.1](https://github.com/micromdm/micromdm/compare/v1.3.0...v1.3.1) July 10 2018

- Update base container to Alpine 3.7 (#437)
- Fix bugs in SCEP enrollment (#451)
- Fix issue with APNS timeouts -- Issue #215 (#446)
- Add device_information and security_info commands with curl API (#448)
- Add support for InstallEnterpriseApplication command (#452)

## [v1.3.0](https://github.com/micromdm/micromdm/compare/v1.2.0...v1.3.0)

### Auto-assigner

- Reorganize/refactor MDM, device, webhook services. #423, #424, #425, #426, #427
- Do not allow `mdmctl config set` without args. #421
- Fix for multiple UDID records. #422
- Added/refactored logging. #405, #425
- Added `-homepage` switch. #420
- Warn about deprecated APNS switches. #412
- Disallow bad TLS configuration with `-tls=false`. #414
- Refactored MDM types. #341, #415
- Added DEP auto-assigner feature. #405
- Fixed bug with authentication error messages. #411
- Added support for querying devices by serial(s). #363
- Added support for triggering a DEP sync via API. #404
- Added support for mdmcert.download directly to `mdmctl` #401
- Reject network MDM user attempts until we add support. #379
- Warn when starting without an API key. #396
- Added tools and documentation for ngrok, curl, and APIs. #392
- Fix issue with MDM command `AvailableOSUpdates` parsing. #368
- Validate APNs Push Certificate Topic. #373
- `mdmctl` now outputs to stdout vs. stderr. #360
- Added common HTTP library `httputil`. #350
- Added project Code of Conduct. #334
- Refactored services (mostly for HA). #348, #349, #351, #352, #353, #354, #355, #359
- Reorganized project layout. #333, #335, #336, #338, #340, #347
- Added support for version API. #327
- Added command response webhook feature. #315
- Added support for supplied `depsim` URL. #318
- Added Dockerfile. #316

## [v1.2.0](https://github.com/micromdm/micromdm/compare/v1.1.0...v1.2.0) October 31 2017

### User Profiles

- Added support for modifying the default enrollment profile.
- Added support for user level profiles.
- Added support for AccountConfiguration during DEP Enrollment. Specified in blueprints
- Addes support for multiple server configs in `mdmctl`.
- Added `mdmctl mdmcert upload` command which uploads/replaces the servers push certificate.
- Incorporated certhelper into mdmctl. See `mdmctl mdmcert -h`
- Added ENV variables for sensitive flags: `MICROMDM_APNS_KEY_PASSWORD`,`MICROMDM_API_KEY`
- Removed the `-redir-addr` flag. Redirect to HTTPS is only enabled when the 443 port is used.

## [v1.1.0](https://github.com/micromdm/micromdm/compare/v1.0.0...v1.1.0) June 05 2017

### YVR!

- Import and sign pkgs, generate appmanifest on import.
- Support syncing devices from DEP when token is added.
- Option to include SSL certificates in DEP profile template (-anchor and -use-server-cert) #107
- /push and /v1/commands API endpoints require API authentication #157
- Add `mdmctl` binary for interacting with the server over API. #127
- Save DEP cursor for use after restart. #109
- Add `-examples` flag to micromdm serve. #119
- Add HTTP logger (with apache format) for all endpoints. #85
- Serve a basic homepage at `/`. #113
- Decrypt armored private keys if the `-apns-password` flag is specified by the user. #105
- Improved command queue handling of NotNow and other responses. #96
- Fixed bug that allowed for duplicate device records on re-enrollment. #125
- Fixed data race in pubsub package. #97
- Fixed bug that would cause PushInfo Token for the device to be replaced by one for the user. #90
