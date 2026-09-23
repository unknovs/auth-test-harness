# OAuth OIDC Mock Service

A mock OAuth 2.0 and OpenID Connect service for testing purposes, implementing the authorization code flow with specific endpoints and user profiles.

## Features

- OAuth 2.0 Authorization Code Flow
- OpenID Connect UserInfo endpoint and discovery document
- In-memory token storage
- Support for multiple authentication methods (Mobile ID, Smart Card and eID Scan)
- A per-method user profile, each with its own name and — optionally — its own
  identity code, so one instance can stand in for two different people
- Configurable through environment variables

## Docker compose

[Docker compose](./docker-compose.yaml) is made to automate eParaksts authentication platform tests. [Postman collection](postman_collection.json) added for this compose as well.  

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
  "expires_in": 600
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
  "acr": "urn:safelayer:tws:policies:authentication:level:high",
  "amr": ["`ACR_VALUES_SUPPORTED` used in request"],
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

The `amr` reports the method that was actually requested: the trailing segment of
the requested flow URN is carried into
`urn:eparaksts:tws:policies:authentication:adaptive:methods:<segment>`. So a client
that forces a specific method can assert it got that method, and a flow this
service was never taught about still reports a well-formed method URN. (The live
platform currently reports `…methods:mobileid` for both the mobile and the eID Scan
flow and distinguishes them only in the `acr`; reporting the requested method is the
more useful behaviour for a test double.)

Each profile may carry **its own identity code**. A system that keys a person on
their identity code then sees a *different person per flow*, which is what lets one
instance stand in for two parties — say a document owner and a counterparty — in a
sharing or co-signing flow. Leave the per-profile variables unset and every method
reports the single `SERIAL_NUMBER`, i.e. the same person however they signed in.

## Environment Variables

### Basic Configuration

- `PORT` - Server port
- `HOST` - Server host
- `BASIC_AUTH_VALUE` - Base64 encoded credentials for token endpoint

### Endpoint Configuration

- `AUTHORIZATION_ENDPOINT` - Authorization endpoint path
- `TOKEN_ENDPOINT` - Token endpoint path
- `USERINFO_ENDPOINT` - UserInfo endpoint path

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
