/**
 * cryptoService — AES-256-GCM helpers for WebSocket message encryption.
 *
 * Wire format matches `excalidraw-be/internal/crypto`:
 *
 *     { iv: <base64 12-byte nonce>, ciphertext: <base64 ciphertext+tag> }
 *
 * Defense in depth on top of TLS. Each room has its own key delivered by
 * the server inside the `encryption_handshake` message right after
 * `join_room`.
 */

export interface EncryptedEnvelope {
  iv: string;
  ciphertext: string;
}

const AES_PARAMS: AesKeyAlgorithm = { name: "AES-GCM", length: 256 };
const IV_BYTES = 12;

/**
 * Imports a base64-encoded 32-byte key into a non-extractable CryptoKey.
 */
export async function importRoomKey(base64Key: string): Promise<CryptoKey> {
  const raw = base64ToBytes(base64Key);
  if (raw.byteLength !== 32) {
    throw new Error(
      `Invalid room key length: got ${raw.byteLength} bytes, want 32`,
    );
  }
  return crypto.subtle.importKey("raw", raw, AES_PARAMS, false, [
    "encrypt",
    "decrypt",
  ]);
}

/**
 * Encrypts plaintext with the room key. Generates a fresh nonce per call.
 */
export async function seal(
  key: CryptoKey,
  plaintext: string,
): Promise<EncryptedEnvelope> {
  // crypto.getRandomValues returns Uint8Array<ArrayBuffer>, satisfying
  // BufferSource without further coercion.
  const iv = crypto.getRandomValues(new Uint8Array(IV_BYTES));
  const data = stringToBytes(plaintext);
  const ct = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, data);
  return {
    iv: bytesToBase64(iv),
    ciphertext: bytesToBase64(new Uint8Array(ct)),
  };
}

/**
 * Decrypts an envelope with the room key. Throws on tag failure.
 */
export async function open(
  key: CryptoKey,
  envelope: EncryptedEnvelope,
): Promise<string> {
  const iv = base64ToBytes(envelope.iv);
  if (iv.byteLength !== IV_BYTES) {
    throw new Error(
      `Invalid IV length: got ${iv.byteLength} bytes, want ${IV_BYTES}`,
    );
  }
  const ct = base64ToBytes(envelope.ciphertext);
  const pt = await crypto.subtle.decrypt({ name: "AES-GCM", iv }, key, ct);
  return new TextDecoder().decode(pt);
}

// ─── base64 / encoding helpers ───────────────────────────────────────────
// All helpers return Uint8Array backed by a fresh ArrayBuffer (not
// SharedArrayBuffer or any aliased view), which is what WebCrypto's
// BufferSource expects under strict TypeScript lib.dom typings.

function stringToBytes(s: string): Uint8Array<ArrayBuffer> {
  const enc = new TextEncoder().encode(s);
  return copyToFreshBuffer(enc);
}

function bytesToBase64(bytes: Uint8Array): string {
  let bin = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    bin += String.fromCharCode(bytes[i]);
  }
  return btoa(bin);
}

function base64ToBytes(b64: string): Uint8Array<ArrayBuffer> {
  const bin = atob(b64);
  const buf = new ArrayBuffer(bin.length);
  const out = new Uint8Array(buf);
  for (let i = 0; i < bin.length; i++) {
    out[i] = bin.charCodeAt(i);
  }
  return out as Uint8Array<ArrayBuffer>;
}

function copyToFreshBuffer(src: Uint8Array): Uint8Array<ArrayBuffer> {
  const buf = new ArrayBuffer(src.byteLength);
  const out = new Uint8Array(buf);
  out.set(src);
  return out as Uint8Array<ArrayBuffer>;
}
