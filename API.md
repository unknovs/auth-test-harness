# OAuth OIDC Mock Service API Documentation

## Overview

This mock service implements OAuth 2.0 Authorization Code Flow with OpenID Connect UserInfo endpoint. It's designed for testing purposes and provides specific user profiles based on authentication methods.

## Base URL

```sh
http://localhost:8080
```

## Authentication Methods

The service supports two authentication methods controlled by the `acr_values` parameter and set as environmental values (two type of users can be defined)

## Endpoints

### 1. Service Information

```http
GET /
```

Returns service metadata and available endpoints.

**Response:**

```json
{
  "service": "OAuth OIDC Mock Service",
  "version": "1.0.0",
  "openid_configuration": "http://localhost:8080/.well-known/openid_configuration",
  "endpoints": {
    "authorize": "as defined in AUTHORIZATION_ENDPOINT environment variable",
    "token": "as defined in TOKEN_ENDPOINT environment variable",
    "userinfo": "as defined in USERINFO_ENDPOINT environment variable",
    "health": "/health"
  },
  "supported_scopes": ["as defined in SCOPES_SUPPORTED environment variable"],
  "supported_acr_values": [
    "as defined in ACR_VALUES_SUPPORTED environment variable"
  ]
}
```

### 2. OpenID Connect Discovery

```http
GET /.well-known/openid_configuration
```

Returns OpenID Connect Discovery information (RFC 8414 compliant).

**Response:**

```json
{
  "issuer": "http://localhost:8080",
  "authorization_endpoint": "as defined in AUTHORIZATION_ENDPOINT environment variable",
  "token_endpoint": "as defined in TOKEN_ENDPOINT environment variable",
  "userinfo_endpoint": "as defined in USERINFO_ENDPOINT environment variable",
  "jwks_uri": "http://localhost:8080/.well-known/jwks.json",
  "scopes_supported": ["as defined in SCOPES_SUPPORTED environment variable"],
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic"],
  "acr_values_supported": [
    "as defined in ACR_VALUES_SUPPORTED environment variable"
  ]
}
```

### 3. Authorization Endpoint

```http
GET [as defined in AUTHORIZATION_ENDPOINT environment variable]
```

Initiates the OAuth authorization flow.

**Query Parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `response_type` | Yes | Must be `code` |
| `client_id` | Yes | Client identifier |
| `redirect_uri` | Yes | Callback URL |
| `scope` | Yes | As defined in SCOPES_SUPPORTED environment variable |
| `state` | Recommended | CSRF protection token |
| `acr_values` | Yes | As defined in ACR_VALUES_SUPPORTED environment variable |
| `prompt` | No | Authentication prompt |
| `ui_locales` | No | UI locale preferences |

**Example Request:**

```http
GET [AUTHORIZATION_ENDPOINT]?response_type=code&client_id=demo_client&state=xyz123&redirect_uri=https://example.com/callback&scope=[SCOPES_SUPPORTED]&acr_values=[ACR_VALUES_SUPPORTED]
```

**Success Response:**

```http
HTTP/1.1 302 Found
Location: https://example.com/callback?code=AUTHORIZATION_CODE&state=xyz123
```

**Error Response:**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "invalid_request",
  "error_description": "Invalid acr_values"
}
```

### 4. Token Endpoint

```http
POST [as defined in TOKEN_ENDPOINT environment variable]
```

Exchanges authorization code for access token.

**Headers:**

```http
Authorization: Basic BASE64(client_id:client_secret)
Content-Type: application/x-www-form-urlencoded
```

**Body Parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `grant_type` | Yes | Must be `authorization_code` |
| `code` | Yes | Authorization code from step 1 |
| `redirect_uri` | Yes | Must match authorization request |

**Example Request:**

```http
POST [TOKEN_ENDPOINT]
Authorization: Basic dGVzdDp0ZXN0
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&code=AUTH_CODE&redirect_uri=https://example.com/callback
```

**Success Response:**

```json
{
  "access_token": "ACCESS_TOKEN",
  "token_type": "Bearer",
  "expires_in": 600
}
```

**Error Response:**

```json
{
  "error": "invalid_grant",
  "error_description": "Invalid or expired authorization code"
}
```

### 5. UserInfo Endpoint

```http
GET [as defined in USERINFO_ENDPOINT environment variable]
```

Returns user information for the authenticated user.

**Headers:**

```http
Authorization: Bearer ACCESS_TOKEN
```

**Success Response:**

if not specific `ACR_VALUES_SUPPORTED`set below info will be responded:

```json
{
  "sub": "UNIQUE_USER_ID",
  "domain": "citizen",
  "acr": "urn:safelayer:tws:policies:authentication:level:high",
  "amr": ["urn:authentication:adaptive:methods:plugin"],
  "given_name": "as defined in SC_GIVEN_NAME environment variable",
  "family_name": "as defined in SC_FAMILY_NAME environment variable",
  "name": "as defined in SC_GIVEN_NAME + SC_FAMILY_NAME environment variables",
  "serial_number": "as defined in SERIAL_NUMBER environment variable",
  "eips": ""
}
```

**Error Response:**

```json
{
  "error": "invalid_token",
  "error_description": "Invalid or expired access token"
}
```

### 6. Health Check

```http
GET /health
```

Returns service health status.

**Response:**

```json
{
  "status": "ok",
  "timestamp": "2025-07-14T14:41:17+03:00"
}
```

## Error Codes

| Error Code | Description |
|------------|-------------|
| `invalid_request` | Missing or invalid required parameters |
| `invalid_client` | Invalid client authentication |
| `invalid_grant` | Invalid or expired authorization code |
| `invalid_scope` | Invalid or unsupported scope |
| `invalid_token` | Invalid or expired access token |
| `unsupported_grant_type` | Grant type not supported |

## Configuration

Environment variables:

| Variable | Description |
|----------|-------------|
| `PORT` | Server port |
| `HOST` | Server host |
| `BASIC_AUTH_VALUE` | Base64 encoded credentials |
| `AUTHORIZATION_ENDPOINT` | Authorization endpoint path |
| `TOKEN_ENDPOINT` | Token endpoint path |
| `USERINFO_ENDPOINT` | UserInfo endpoint path |
| `SCOPES_SUPPORTED` | Comma-separated list of supported scopes |
| `ACR_VALUES_SUPPORTED` | Comma-separated list of supported ACR values |
| `SERIAL_NUMBER` | Serial number for user profiles |
| `MOBILE_GIVEN_NAME` | Given name for Mobile user |
| `MOBILE_FAMILY_NAME` | Family name for Mobile user |
| `SC_GIVEN_NAME` | Given name for Smart Card user |
| `SC_FAMILY_NAME` | Family name for Smart Card user |

## Token Lifetimes

- **Authorization codes**: 10 minutes
- **Access tokens**: 10 minutes (600 seconds)

## Security Notes

- This is a mock service for testing only
- All tokens are stored in memory
- No real authentication is performed
- Basic authentication uses hardcoded/set by environment credentials

## Example Flow

1. **Get Authorization Code:**

   ```bash
   curl -i "http://localhost:8080[AUTHORIZATION_ENDPOINT]?response_type=code&client_id=demo&state=xyz&redirect_uri=https://example.com/callback&scope=[SCOPES_SUPPORTED]&acr_values=[ACR_VALUES_SUPPORTED]"
   ```

2. **Extract code from redirect URL**

3. **Get Access Token:**

   ```bash
   curl -X POST http://localhost:8080[TOKEN_ENDPOINT] \
     -H "Authorization: Basic dGVzdDp0ZXN0" \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "grant_type=authorization_code&redirect_uri=https://example.com/callback&code=YOUR_CODE"
   ```

4. **Get User Info:**

   ```bash
   curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
     http://localhost:8080[USERINFO_ENDPOINT]
   ```
