# JWT Configuration

Authara issues short-lived JWT access tokens signed with Ed25519 (`alg=EdDSA`).
Core holds the private signing seeds; applications and SDKs validate tokens
with public keys from Authara's JWKS endpoint. A relying application never
needs access to private signing material.

See also: [Configuration Reference](reference.md)

## Configuration

```env
AUTHARA_JWT_ISSUER=authara
AUTHARA_JWT_ACTIVE_KEY_ID=key-2026-01
AUTHARA_JWT_PRIVATE_KEYS=key-2026-01:BASE64_ED25519_SEED
AUTHARA_ACCESS_TOKEN_TTL_MINUTES=10
```

`AUTHARA_JWT_ISSUER` becomes the token's `iss` claim. Validators must compare
it exactly with their configured expected issuer.

`AUTHARA_JWT_ACTIVE_KEY_ID` selects the key used for new tokens. It must exist
in `AUTHARA_JWT_PRIVATE_KEYS`, and its ID is included in the JWT header as
`kid`.

`AUTHARA_JWT_PRIVATE_KEYS` is a comma-separated map:

```text
keyID:base64Ed25519Seed,keyID2:base64Ed25519Seed2
```

Every seed must decode to exactly 32 bytes. Generate one with:

```bash
openssl rand -base64 32
```

Keep private seeds in a secret manager, use different keys per environment,
and never provide this setting to an application or SDK.

## Public JWKS

Core publishes the corresponding public keys at:

```text
GET /auth/.well-known/jwks.json
```

Each JWK is an `OKP`/`Ed25519` verification key with `alg=EdDSA`, `use=sig`,
and the matching `kid`. The response contains no private `d` value. It is
publicly cacheable for five minutes and supports conditional requests with an
ETag.

Configure an SDK with Authara's base URL or the explicit JWKS URL. Validation
must still check the signature, `kid`, algorithm, issuer, audience, expiry,
issued-at time, and required Authara claims. JWKS is key metadata, not an
introspection endpoint and not an OpenID Connect discovery document.

## Key rotation

Rotation overlaps the old and new public keys. For the smoothest rollout,
publish the new key before using it:

1. Generate a new 32-byte seed.
2. Add it to `AUTHARA_JWT_PRIVATE_KEYS` without changing the active key, then
   restart Core.
3. Wait at least the five-minute JWKS cache lifetime so validators learn the
   new public key.
4. Set `AUTHARA_JWT_ACTIVE_KEY_ID` to the new ID and restart Core again.
5. Keep the old key configured for at least the maximum access-token lifetime,
   plus cache and clock-skew allowance.
6. Remove the old key and restart Core after that window.

For example:

```env
AUTHARA_JWT_ACTIVE_KEY_ID=key-2026-06
AUTHARA_JWT_PRIVATE_KEYS=key-2026-06:BASE64_NEW_SEED,key-2026-01:BASE64_OLD_SEED
```

SDKs normally use the cached JWKS. When a token arrives with a new `kid`, the
Authara Go SDK performs one rate-limited refresh immediately, so rollout does
not have to wait for normal cache expiry.

Do not remove the old key at the same time you activate the new one. An
already-issued token remains valid until expiry and must retain a matching
public key throughout that bounded lifetime.

### Rotation with multiple Core replicas

Every Core replica behind a load balancer must use the same private-key set and
active key. A replica that signs with a new key while another replica still
publishes only the old JWKS can create tokens that validators intermittently
reject, depending on which replica serves the JWKS request.

Use two fleet-wide rollouts:

1. Add the new private key to every replica while the old key remains active.
2. Confirm every replica publishes both public keys, then wait at least the
   five-minute JWKS cache lifetime.
3. Change the active key to the new ID on every replica, keeping both keys.
4. After the old-token lifetime, JWKS cache lifetime, and clock-skew allowance
   have elapsed, remove the old key from every replica.

Do not activate a key during the first rolling deployment. Complete and verify
the fleet-wide prepublication step before any replica signs with the new key.

## Token lifetime and revocation

`AUTHARA_ACCESS_TOKEN_TTL_MINUTES` defaults to 10 and may be between 1 and
1440 minutes. Expiry-only revocation mode imposes the stricter 10-minute
maximum. With immediate revocation, SDK middleware can additionally query the
shared Redis revocation records after local signature verification.

JWKS changes only how the signature is verified. It does not replace Authara's
existing session, refresh-token, or Redis revocation behavior.

## Migration from shared HS256 keys

This release removes HS256 access-token signing and the
`AUTHARA_JWT_KEYS` setting. Replace it with `AUTHARA_JWT_PRIVATE_KEYS`, and
configure relying services with Authara's base URL or JWKS URL instead of a
shared secret.

If Core and relying services are deployed independently, release the
JWKS-capable SDK first, update each service, and then deploy the EdDSA-signing
Core version. There is intentionally no dual-algorithm period: accepting both
HS256 and EdDSA would preserve the secret-sharing design and widen the
algorithm-confusion surface. Because no production consumers exist yet, the
cleanest rollout is a coordinated breaking deployment.
