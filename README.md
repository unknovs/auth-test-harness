# OAuth OIDC Mock Service

A mock OAuth 2.0 and OpenID Connect service for testing purposes, implementing the authorization code flow with specific endpoints and user profiles.

## Features

- OAuth 2.0 Authorization Code Flow
- OpenID Connect UserInfo endpoint and discovery document
- In-memory token storage
- Support for multiple authentication methods (Mobile ID, Smart Card and eID Scan)
- A per-method user profile, each with its own name and — optionally — its own
  identity code, so one instance can stand in for two different people
- An identity list (`USED_IDENTITIES=list`): Mobile ID and eID Scan ask for a personal
  code and answer as the person the list names for it, so any number of test people
  can sign in: twenty made-up people built in, or your own mounted file, changed without a restart
- The identity provider's logout endpoint
- Configurable through environment variables

How a sign-in runs in each identity mode, as sequence diagrams: [FLOWS](FLOWS.md). What changed between
releases: [CHANGELOG](CHANGELOG.md).

## Docker compose

[Docker compose](./docker-compose.yaml) is made to automate eParaksts authentication platform tests. [Postman collection](postman_collection.json) added for this compose as well.  

It runs the service twice: `auth-test-harness` on port 8080 with the configured profiles,
and `auth-test-harness-list` on port 8081 in `list` mode, on the built-in
[identity list](examples/identities/identities.json) (see [The identity list](#the-identity-list)).

## Docker image

Docker image available on [DockerHub](https://hub.docker.com/r/unknovs/oauth-oidc-mock-service)

## Endpoints

### 1. Authorization Endpoint

```sh
GET [`AUTHORIZATION_ENDPOINT`]
```

**Parameters:**

- `response_type=code` (required)
- `client_id` (required)
- `state` (optional but recommended)
- `redirect_uri` (required)
- `scope` - one of defined in `SCOPES_SUPPORTED` environment variable (required)
- `prompt` (optional)
- `acr_values` - one of defined in `ACR_VALUES_SUPPORTED` environment variable (required)
- `ui_locales` (optional)
- `nonce` (optional) - carried back inside the `id_token` the code is exchanged for, so the client can bind that token to this request

**Supported ACR Values:**

- Defines in `ACR_VALUES_SUPPORTED` environment variable. Besides the eParaksts-shaped flows, two
  **directory flows** are understood when listed there: `urn:auth-test-harness:flow:directory` — a
  person signing in with a work account at their organisation's directory (a member), and
  `urn:auth-test-harness:flow:directory-guest` — the same person flagged as a guest of that directory.
  A directory profile carries a name and a durable object id (`oid`) and **no identity code**; its
  `amr` is `urn:auth-test-harness:methods:directory`, so a client's method vocabulary can map it.

**Response:**

With an identity list in use (`USED_IDENTITIES=list`), Mobile ID and eID Scan first answer the
sign-in page that asks for a personal code ([The identity list](#the-identity-list)); the redirect
below follows once a listed code is entered. Every other flow redirects at once.

Redirects to `redirect_uri` with `code` and `state` parameters.

### 2. Token Endpoint

```sh
POST [`TOKEN_ENDPOINT`]
```

**Headers:**

- `Authorization: Basic {base64_encoded_credentials}`
- `Content-Type: application/x-www-form-urlencoded`

**Body Parameters:**

- `grant_type=authorization_code`
- `redirect_uri` (must match the one used in authorization)
- `code` (authorization code from step 1)

**Response:**

```json
{
  "access_token": "string",
  "token_type": "Bearer",
  "expires_in": 600,
  "scope": "the scope the authorization request asked for",
  "id_token": "…"
}
```

### 3. UserInfo Endpoint

```sh
GET [`USERINFO_ENDPOINT`]
```

**Headers:**

- `Authorization: Bearer {access_token}`

**Response:**

```json
{
  "sub": "`UNIQUE_USER_ID`",
  "domain": "citizen",
  "acr": "the requested `acr_values` (the flow), echoed back",
  "amr": ["urn:safelayer:tws:policies:authentication:adaptive:methods:mobileid"],
  "given_name": "as defined in `SC_GIVEN_NAME` (or `MOBILE_GIVEN_NAME`) environment variable",
  "family_name": "as defined in `SC_FAMILY_NAME` (or `MOBILE_FAMILY_NAME`) environment variable",
  "name": "as defined in given_name + family_name environment variables",
  "serial_number": "as defined in `SERIAL_NUMBER` environment variable",
  "eips": ""
}
```

**User Profiles** — selected by the `acr_values` the client requested:

- Mobile ID (`urn:eparaksts:authentication:flow:mobileid`): `MOBILE_GIVEN_NAME`, `MOBILE_FAMILY_NAME`, `MOBILE_SERIAL_NUMBER`
- Smart Card (`urn:eparaksts:authentication:flow:sc_plugin`, and any other requested flow): `SC_GIVEN_NAME`, `SC_FAMILY_NAME`, `SC_SERIAL_NUMBER`
- eID Scan (`urn:eparaksts:authentication:flow:mobile-eid`): `EIDSCAN_GIVEN_NAME`, `EIDSCAN_FAMILY_NAME`, `EIDSCAN_SERIAL_NUMBER` (names fall back to the Smart Card ones — both are card-based)

The `acr` and `amr` are what the eParaksts platform answers:

- the **`acr` echoes the requested flow** (for example
  `urn:eparaksts:authentication:flow:mobile-eid`) — an authentication context, not a level of
  assurance;
- the **`amr` is `urn:safelayer:tws:policies:authentication:adaptive:methods:mobileid` for both
  Mobile ID and eID Scan**: eID Scan rides the same mechanism, and the platform reports it so. A
  client tells the two apart by the `acr` alone, as it must against the platform;
- for the smart card, and any other flow, the `amr` is the documented
  `urn:eparaksts:tws:policies:authentication:adaptive:methods:<segment>`, the segment carried from
  the requested flow (the platform's answer for these has not been measured).

Each profile may carry **its own identity code**. A system that keys a person on
their identity code then sees a *different person per flow*, which is what lets one
instance stand in for two parties — say a document owner and a counterparty — in a
sharing or co-signing flow. Leave the per-profile variables unset and every method
reports the single `SERIAL_NUMBER`, i.e. the same person however they signed in.

### 4. Logout Endpoint

```sh
GET [`LOGOUT_ENDPOINT`]?redirect_uri=...
```

The identity provider's session-termination endpoint, served when `LOGOUT_ENDPOINT` is set (the
eParaksts path is `/trustedx-authserver/lvrtc-eipsign-idp/logout`). It redirects (`302`) the browser
to `redirect_uri`. The service keeps no sign-in session — every authorization request is answered
afresh — so there is nothing else to end, and access tokens already issued stay valid until they
expire. A missing or relative `redirect_uri` is refused with `invalid_request`.

## The identity list

With `USED_IDENTITIES=list`, the flows that take a personal code — **Mobile ID and eID Scan** — ask
for it and answer as the person the identity list names for it. So any number of test people can sign
in, and one tester can sign in as any of them.

**Nothing to manage by default.** With no `IDENTITIES_FILE`, the service uses its **built-in list of
twenty made-up people**, `PNOLV-000123-00001` to `PNOLV-000123-00020`
([`examples/identities/identities.json`](examples/identities/identities.json), compiled in). Set
`IDENTITIES_FILE` only when you need specific people, such as the codes of your own test accounts; your
file then replaces the built-in list. The smart-card flow and the directory flows take no code
and keep their configured profiles.

**The sign-in page.** The authorization request for these flows is answered with a page holding one
field, the personal code. It is posted to `/identify`; a listed code redirects to the client with an
authorization code, exactly as the authorization endpoint does in the other flows, and the login
answers as that person. The code may be typed as the full serial number (`PNOLV-…`) or without its
prefix, in any letter case. A code the list does not name shows the page again, so it can be
corrected.

**The format**, of the built-in list and of your own file at the path `IDENTITIES_FILE` sets:

```json
{
  "identities": [
    { "serial_number": "PNOLV-000123-00001", "given_name": "Anna", "family_name": "Paraudziņa" },
    { "serial_number": "PNOLV-000123-00002", "given_name": "Jānis", "family_name": "Paraugs" }
  ]
}
```

Each entry needs all three fields, a serial number appears once, and a field the format does not
know is refused (so a misspelt name is reported, not silently dropped). The built-in list's codes are
chosen to be no one's: the first six digits of a Latvian personal code carrying a date of birth are that
date, and `000123` is none (day 00).

**Changing your file while the service runs.** The file is checked at every code entry and read again when
it has changed — no restart. A file that cannot be used (malformed, a repeated code, a missing field)
is refused with a log line naming the problem, and **the last good list stays in use**, so an edit
with a typo in it does not take every login down. A file missing or unusable at start is logged too;
no one can sign in from the list until it is fixed. **Mount the directory, not the file**
(`./identities:/identities:ro`): an editor that saves by writing a new file leaves a single-file bind
mount pointing at the old one, and Kubernetes does not refresh a ConfigMap mounted with `subPath`.

**The subject.** A listed person's `sub` derives from the method and their code: the same on every
login by one method, and after their names are edited in the list, but different by Mobile ID and
by eID Scan — as at the platform, where one person signs in under a different subject by each
method. (A configured profile's subject is per profile, as described in the `id_token` section.)

## Environment Variables

### Basic Configuration

- `PORT` - Server port
- `HOST` - Server host
- `BASIC_AUTH_VALUE` - Base64 encoded credentials for token endpoint

### Endpoint Configuration

- `AUTHORIZATION_ENDPOINT` - Authorization endpoint path
- `TOKEN_ENDPOINT` - Token endpoint path
- `USERINFO_ENDPOINT` - UserInfo endpoint path
- `LOGOUT_ENDPOINT` - Logout endpoint path (optional; unset, no logout endpoint is served)

### Identity Source

- `USED_IDENTITIES` - `config` (the default: the user profiles below, one per flow) or `list` (Mobile
  ID and eID Scan answer as the person whose code is entered). Any other value stops the service at
  start.
- `IDENTITIES_FILE` - Optional, with `list`: the path of your own identity list (for example
  `/identities/identities.json`, its directory mounted). Unset, the built-in list of twenty made-up
  people is used.

### Supported Values Configuration

- `SCOPES_SUPPORTED` - Comma-separated list of supported scopes
- `ACR_VALUES_SUPPORTED` - Comma-separated list of supported ACR values

### User Profile Configuration

- `SERIAL_NUMBER` - Identity code every profile reports unless it overrides it below
- `MOBILE_GIVEN_NAME` / `MOBILE_FAMILY_NAME` / `MOBILE_SERIAL_NUMBER` - Mobile ID user
- `SC_GIVEN_NAME` / `SC_FAMILY_NAME` / `SC_SERIAL_NUMBER` - Smart Card user
- `EIDSCAN_GIVEN_NAME` / `EIDSCAN_FAMILY_NAME` / `EIDSCAN_SERIAL_NUMBER` - eID Scan user
  (names default to the Smart Card ones; the identity code defaults to `SERIAL_NUMBER`)
- `DIRECTORY_GIVEN_NAME` / `DIRECTORY_FAMILY_NAME` / `DIRECTORY_OBJECT_ID` - the directory user
  (defaults `Ilze` / `Ozola`; the object id defaults to a value derived from the names, so it is
  stable across restarts). This profile carries **no identity code** — a directory holds none.

### The `id_token` and the key set

Every token response carries an `id_token`: a JWT signed RS256 with a key generated when the
service starts and published at `/.well-known/jwks.json` (the `jwks_uri` the discovery document
advertises). It names the issuer (`PROTOCOL://HOST`), the client that asked (`aud`), the profile's
subject, the request's `nonce`, the names and the method (`amr`); a directory profile's token also
carries `oid` (the durable object id) and `acct` (`0` a member, `1` a guest) — where a directory puts
them, so the userinfo answer carries neither. A restart rotates the key, which is what a provider's
key rotation looks like to a client.

**The subject is stable.** A profile's `sub` is the same on every login and differs between profiles
(the guest variant is the same person and keeps it), so a client that stores a credential under the
subject — or checks that the `id_token` and userinfo name the same person — sees one person, not a
new one per login.

## Usage

### Running the Service

```bash
# Set environment variables (configure as needed)
export PORT=8080
export HOST=localhost:8080
export BASIC_AUTH_VALUE=[your_base64_credentials]
export AUTHORIZATION_ENDPOINT=[your_auth_endpoint]
export TOKEN_ENDPOINT=[your_token_endpoint]
export USERINFO_ENDPOINT=[your_userinfo_endpoint]
export SCOPES_SUPPORTED=[your_supported_scopes]
export ACR_VALUES_SUPPORTED=[your_supported_acr_values]

# Run the service
go run main.go
```

### Example OAuth Flow

1. **Authorization Request:**

    ```sh
    GET http://localhost:8080[AUTHORIZATION_ENDPOINT]?response_type=code&client_id=test_client&state=xyz&redirect_uri=https://www.demoapp.lv/oauth/back&scope=[SCOPES_SUPPORTED]&acr_values=[ACR_VALUES_SUPPORTED]
    ```

2. **Token Request:**

    ```bash
    curl -X POST http://localhost:8080[TOKEN_ENDPOINT] \
    -H "Authorization: Basic [BASIC_AUTH_VALUE]" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=authorization_code&redirect_uri=https://www.demoapp.lv/oauth/back&code=YOUR_AUTH_CODE"
    ```

3. **UserInfo Request:**

    ```bash
    curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
    http://localhost:8080[USERINFO_ENDPOINT]
    ```

## Health Check

```sh
GET /health
```

Returns service health status.
