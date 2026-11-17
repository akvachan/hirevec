# Security

## TLS

TLS is disabled by default, because:

- You may run this app via a reverse proxy that already enables TLS communication.
- You are developing and don't want to experience the burden of TLS-related infra, including setting up a proper CA and securing local certificate management.

You can enable TLS via `HIREVEC_TLS_ENABLED=true`. You must then provide `HIREVEC_TLS_CERT_FILE`
and `HIREVEC_TLS_KEY_FILE`. Make sure that these files are configured to have 0o600 permissions,
such that only owner can read and write.

## User authorization

Each user has their own [PASETO](https://github.com/paseto-standard/paseto-spec) access token.
Access tokens are transient (30 minutes by default). After expiration, access tokens can be obtained
via PASETO refresh tokens. Refresh tokens are obtained upon user creation, registration or login completion.
Refresh tokens are also transient (2 years by default). A new refresh token is issued on every login
and on every refresh-token rotation. When a refresh token expires, the user is logged out
automatically. A user can have up to 5 refresh tokens simultaneously, after which the oldest one gets
revoked automatically. A consequence of this is that there may be only up to 5 concurrent
sessions per user at a time. Reuse of a consumed token revokes the whole token family.

Tokens are not bound to an IP address or a user agent: a stolen access token works from anywhere
until it expires, and a stolen refresh token works until it is rotated or reused (reuse detection
revokes the whole token family).

Access tokens are PASETO v4.public tokens: they are signed, but readable by anyone who obtains one,
so no secrets must be placed in token claims. Refresh tokens are PASETO v4.local tokens: they are
encrypted via a symmetric key, which can be set via `HIREVEC_SYMMETRIC_KEY_FILE`.
Access tokens are signed via an asymmetric key, which can be set via `HIREVEC_ASYMMETRIC_KEY_FILE`.
If key files are not provided, server generates default keys and stores them in `~/.hirevec/.symkey.txt`
and `~/.hirevec/.asymkey.txt` with 0o600 permissions, such that only owner can read and write.

Both token types are issued with the issuer and the audience set to `hirevec.com`. The values are
validated by the server itself and are not derived from the deployment domain.

## User authentication

A user's identity is anchored to a [WebAuthn](https://en.wikipedia.org/wiki/WebAuthn)
[passkey](https://en.wikipedia.org/wiki/Passkey). Passkeys are used to obtain initial access token
and refresh token via following flow:

1. Server generates a challenge and other parameters.
2. Inside the browser, user creates or retrieves a passkey.
3. Server validates the challenge and other parameters.

Login is discoverable and usernameless: the user never types an email to sign in, the authenticator
identifies the account. Registration requires an email and a full name. An account has exactly one
passkey: there are currently no endpoints to add or remove credentials, so a passkey account that
loses its passkey is unreachable, mirroring the refresh token loss described below.

Ceremony challenges are single-use and stored server-side with a 5-minute expiry. Expired ceremonies
are cleaned up every 15 minutes. Sign counters reported by authenticators are tracked per credential.
The server validates the ceremony origin and the relying party ID on every ceremony.

A minimal page for performing both ceremonies is served at `/passkeys`. It is a static HTML page
that calls the same public endpoints a custom frontend would.

### WebAuthn relying party configuration

The relying party ID defaults to the host of `HIREVEC_URL` and the allowed origins default to the
scheme and host of `HIREVEC_URL`. They can be overridden via `HIREVEC_WEBAUTHN_RPID` and
`HIREVEC_WEBAUTHN_RP_ORIGINS` (comma-separated). A ceremony is rejected when the browser origin or
the relying party ID does not match, so the passkey page must be served and opened on the configured
origin. When the server runs behind a reverse proxy, `HIREVEC_URL` must reflect the public origin.

**This server does not support MFA, OTP, OIDC or passwords**. Users must have either a
FIDO2-compliant security key or a password manager, such as [Apple Passwords](https://en.wikipedia.org/wiki/Passwords_(Apple))
or [1Password](https://en.wikipedia.org/wiki/1Password). Works out-of-the-box on all Apple devices.

However, an anonymous user can also be created via `POST /users` (when the API key is not enabled).
Such an anonymous user enjoys the same privileges as a passkey-registered user: both start without
roles and can assign themselves the candidate or recruiter role via `POST /candidates` or `POST /recruiters`.
Such an account has no passkey, and since signup is the only event that issues a refresh token,
the user holds exactly one live refresh token at any time.
If it is lost, the account is unreachable: re-registration fails with a conflict, and no passkey exists to sign in with.

## API key

Server API key is disabled by default, but can be enabled by providing `HIREVEC_API_KEY_FILE`
(the key is enabled when the file exists and is non-empty). This is an additional (and weakest) security layer
that is used in combination with TLS and user auth. If enabled, all endpoints that opt into the API key
middleware will require a static `X-API-Key` in the HTTP request header, compared in constant time.
The endpoints that do not opt in are: `/health`, `/docs`, `/openapi.yaml`, `/passkeys` and 
`/webauthn/*`. Do not rely on this authentication method. 
**If TLS is disabled, this authentication method is rendered useless**, since anyone can extract
API key from the requests. Make sure that the key file is configured to have 0o600 permissions, such that
only owner can read and write.

## CORS

The server intentionally sets no CORS headers (`Access-Control-Allow-Origin` et al.). Cross-origin browser
access is therefore rejected by browsers themselves: the API is same-origin-only by design. If you deploy a
browser-based frontend on a different origin, you must deliberately add CORS support. Prefer restricting
it to the exact frontend origin instead of allowing `*`.

## Client IP logging

Request logs include the client IP. By default it is the TCP peer address of the incoming connection, so
behind a reverse proxy every request would be attributed to the proxy. Set `HIREVEC_TRUSTED_PROXY=true`
only when the server is reached through a trusted reverse proxy that appends to `X-Forwarded-For`;
with a single trusted proxy the rightmost `X-Forwarded-For` entry is used as the client IP. The header is
never trusted without this flag because clients can spoof it.

## Threat model notes

- There is currently no rate limiting. When the API key is disabled, anonymous clients can create an
  unlimited number of users. Freshly created users hold no roles, so no private data is exposed to them,
  but the database growth is unbounded.
- The candidate and recruiter roles are self-assigned by any authenticated user; the authorization
  layer treats having an account as the only real boundary.
