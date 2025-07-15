# OAuth OIDC Mock Service

A mock OAuth 2.0 and OpenID Connect service for testing purposes, implementing the authorization code flow with specific endpoints and user profiles.

## Features

- OAuth 2.0 Authorization Code Flow
- OpenID Connect UserInfo endpoint
- In-memory token storage
- Support for multiple authentication methods (Mobile ID and Smart Card)
- Configurable through environment variables

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

**Supported ACR Values:**

- Defines in `ACR_VALUES_SUPPORTED` environment variable

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

**User Profiles:**

- Mobile ID (when `ACR_VALUES_SUPPORTED` contain `urn:eparaksts:authentication:flow:mobileid`): Uses `MOBILE_GIVEN_NAME` and `MOBILE_FAMILY_NAME`
- Smart Card (when `ACR_VALUES_SUPPORTED` contain `urn:eparaksts:authentication:flow:sc_plugin`): Uses `SC_GIVEN_NAME` and `SC_FAMILY_NAME`

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

- `SERIAL_NUMBER` - Serial number for user profiles
- `MOBILE_GIVEN_NAME` - Given name for Mobile ID user
- `MOBILE_FAMILY_NAME` - Family name for Mobile ID user
- `SC_GIVEN_NAME` - Given name for Smart Card user
- `SC_FAMILY_NAME` - Family name for Smart Card user

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
