import { createPublicKey, verify } from "node:crypto";

const chunks = [];
for await (const chunk of process.stdin) {
  chunks.push(chunk);
}

const input = JSON.parse(Buffer.concat(chunks).toString("utf8"));
const parts = input.token.split(".");
if (parts.length !== 3) {
  throw new Error("token must have three segments");
}

const header = JSON.parse(Buffer.from(parts[0], "base64url").toString("utf8"));
if (header.alg !== "EdDSA" || typeof header.kid !== "string") {
  throw new Error("token header must contain EdDSA and kid");
}

const jwk = input.jwks.keys.find((candidate) => candidate.kid === header.kid);
if (!jwk) {
  throw new Error("token kid is missing from JWKS");
}
if (jwk.kty !== "OKP" || jwk.crv !== "Ed25519" || jwk.alg !== "EdDSA") {
  throw new Error("JWKS contains an unexpected key type");
}
if (Object.hasOwn(jwk, "d")) {
  throw new Error("JWKS exposed private key material");
}

const publicKey = createPublicKey({ key: jwk, format: "jwk" });
const signatureValid = verify(
  null,
  Buffer.from(`${parts[0]}.${parts[1]}`),
  publicKey,
  Buffer.from(parts[2], "base64url"),
);
if (!signatureValid) {
  throw new Error("signature verification failed");
}

const claims = JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8"));
const audiences = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
if (claims.iss !== input.issuer || !audiences.includes(input.audience)) {
  throw new Error("issuer or audience mismatch");
}

process.stdout.write("ok\n");
