# Changelog

What changed for anyone running the service or testing against it. A section is headed with the release
tag it ships as. 1.0.0 is the service as first published, untagged; the image built from `main` reports
that version in its service information.

## v2.0.0

The answers of the eParaksts-shaped flows now match the eParaksts identity platform as it was measured,
quirks included: a client that works against this service works against the platform. An identity list
lets any number of made-up people sign in, and the identity provider's logout endpoint is served.

### Changed — the answers now match the platform

**The `acr` is the requested flow, echoed back** (was always
`urn:safelayer:tws:policies:authentication:level:high`). The platform answers its authentication context
here, not a level of assurance. This applies to every eParaksts-shaped flow; the directory flows still
carry no `acr`.

**Mobile ID and eID Scan report the same `amr`:**
`urn:safelayer:tws:policies:authentication:adaptive:methods:mobileid` (Mobile ID was
`urn:eparaksts:tws:policies:authentication:adaptive:methods:mobileid`; the unreleased `develop` image
answered eID Scan with `…:mobile-eid`). eID Scan rides the same mechanism on the platform, and the platform
reports it so. **A client tells the two methods apart by the `acr` alone.** The smart card keeps its documented `amr`
(`urn:eparaksts:tws:policies:authentication:adaptive:methods:sc_plugin`).

userinfo for a Mobile ID login, before and after:

```jsonc
// before
{ "acr": "urn:safelayer:tws:policies:authentication:level:high",
  "amr": ["urn:eparaksts:tws:policies:authentication:adaptive:methods:mobileid"], … }
// after
{ "acr": "urn:eparaksts:authentication:flow:mobileid",
  "amr": ["urn:safelayer:tws:policies:authentication:adaptive:methods:mobileid"], … }
```

An eID Scan login now answers `"acr": "urn:eparaksts:authentication:flow:mobile-eid"` with the same `amr`.

**The token answer carries the granted `scope`**, as the platform's does: the scope the authorization
request asked for.

```json
{ "access_token": "…", "token_type": "Bearer", "expires_in": 600,
  "scope": "urn:lvrtc:fpeil:aa", "id_token": "…" }
```

**The `sub` is stable** (it was new on every userinfo call): the same on every login of a profile,
different between profiles. A person signs in under a different subject by each method, as on the
platform.

**Discovery is served at `/.well-known/openid-configuration`**, the path OpenID Connect defines. The
underscore path the service first shipped still answers the same document.

### Added

**An identity list** — `USED_IDENTITIES=list`. Mobile ID and eID Scan then answer the
authorization request with a sign-in page that asks for a personal code. The code is posted to `/identify`,
and the login answers as the person the list names for it; a code the list does not name shows the page
again. The smart card and the directory flows keep their configured profiles. `USED_IDENTITIES=config`, the
default, keeps the profiles from the environment as before.

- **Nothing to manage by default:** the service carries a built-in list of twenty made-up people,
  `PNOLV-000123-00001` to `-00020` (`examples/identities/identities.json`, compiled in).
- **Your own people:** `IDENTITIES_FILE` names a JSON file,
  `{"identities": [{"serial_number", "given_name", "family_name"}, …]}`, which replaces the built-in list.
  It is read again whenever it changes, with no restart. A file that cannot be used is refused with a log
  line, and the last good list stays in use. **Mount the directory, not the file.**
- A code may be typed in full (`PNOLV-…`) or without its prefix.
- A listed person's `sub` derives from the method and their code, so it survives a rename.
- The compose file runs a second service in `list` mode on port 8081, on the built-in list.

**The logout endpoint** — `LOGOUT_ENDPOINT`, optional (the eParaksts path is
`/trustedx-authserver/lvrtc-eipsign-idp/logout`). `GET …?redirect_uri=…` redirects there. The service keeps
no sign-in session, so access tokens already issued stay valid until they expire. A missing or relative
`redirect_uri` is refused with `invalid_request`.

**The eID Scan flow** — `urn:eparaksts:authentication:flow:mobile-eid`, with its own profile
(`EIDSCAN_GIVEN_NAME`, `EIDSCAN_FAMILY_NAME`, `EIDSCAN_SERIAL_NUMBER`).

**An identity code per profile** — `MOBILE_SERIAL_NUMBER`, `SC_SERIAL_NUMBER`, `EIDSCAN_SERIAL_NUMBER`, each
defaulting to `SERIAL_NUMBER`, so one instance can stand in for two different people.

**An `id_token` on every token answer**, signed RS256 with a key generated at start and published at
`/.well-known/jwks.json`. It carries the issuer, the client, the subject, the request's `nonce`, the
names and the method.

**Two directory flows** — `urn:auth-test-harness:flow:directory` and `…:directory-guest`: a person signing in
with a work account at their organisation's directory, as a member or as a guest. A name and a durable
object id (`oid`, in the `id_token`), and no identity code.

### Fixed

**Concurrent requests no longer race on the in-memory store.** The server answers requests concurrently,
and the clean-up ticker sweeps the same store meanwhile, with no lock between them; concurrent logins could
corrupt it.

### Other

- Go 1.27.2 as the minimum (`go.mod`). Under Go 1.27.0 `govulncheck` finds seven standard-library
  vulnerabilities this service's code reaches, GO-2026-6599, -6600, -6603, -6611, -6612, -6613 and -6617, in
  `net/http` and `html/template`; under 1.27.2 it finds none. CI asks for `1.27` and the image builds from
  `golang:1.27-alpine`, both of which take the newest 1.27 release; the minimum now refuses an older toolchain or
  a stale cached image instead of building a vulnerable binary with it.
- Tests cover every endpoint, including the refusals, and run with the race detector in CI.
- A `golangci-lint` configuration; the image build caches Go modules and builds.
- `FLOWS.md`: the sign-in in `config` and `list` mode, and logout, as sequence diagrams with each step.

## v1.0.0

The service as first published: the OAuth 2.0 authorization code flow with the eParaksts endpoint paths
configurable, the token and userinfo endpoints, a discovery document, a health check, and one profile each
for Mobile ID and the smart card, chosen by `acr_values`.
