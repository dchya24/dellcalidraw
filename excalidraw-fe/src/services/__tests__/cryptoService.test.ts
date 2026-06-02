import { describe, it, expect } from "vitest";
import { importRoomKey, seal, open } from "../cryptoService";

// AES-256 base64 key fixture (32 bytes, all zeros). Real keys are
// generated server-side; using a fixed key in tests keeps the harness
// deterministic.
const ZERO_KEY_BASE64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

function makeRandomKeyBase64(): string {
  const raw = crypto.getRandomValues(new Uint8Array(32));
  let bin = "";
  for (let i = 0; i < raw.length; i++) bin += String.fromCharCode(raw[i]);
  return btoa(bin);
}

describe("cryptoService", () => {
  describe("importRoomKey", () => {
    it("imports a valid 32-byte base64 key", async () => {
      const key = await importRoomKey(ZERO_KEY_BASE64);
      expect(key.algorithm.name).toBe("AES-GCM");
      expect(key.usages).toContain("encrypt");
      expect(key.usages).toContain("decrypt");
    });

    it("rejects keys of the wrong length", async () => {
      const tooShort = btoa("only-16-bytes!!!"); // 16 bytes
      await expect(importRoomKey(tooShort)).rejects.toThrow(
        /Invalid room key length/,
      );
    });

    it("rejects malformed base64", async () => {
      await expect(importRoomKey("not base64!!!")).rejects.toThrow();
    });
  });

  describe("seal + open round-trip", () => {
    it("recovers the original plaintext", async () => {
      const key = await importRoomKey(makeRandomKeyBase64());
      const plain = JSON.stringify({
        type: "update_elements",
        payload: { roomId: "r1", changes: { added: [], updated: [], deleted: [] } },
      });
      const env = await seal(key, plain);

      // Envelope shape matches the BE wire format
      expect(env.iv).toMatch(/^[A-Za-z0-9+/=]+$/);
      expect(env.ciphertext).toMatch(/^[A-Za-z0-9+/=]+$/);

      const out = await open(key, env);
      expect(out).toBe(plain);
    });

    it("uses a fresh nonce per call", async () => {
      const key = await importRoomKey(makeRandomKeyBase64());
      const a = await seal(key, "hello");
      const b = await seal(key, "hello");
      expect(a.iv).not.toBe(b.iv);
      // Same plaintext + different IV must produce different ciphertext
      expect(a.ciphertext).not.toBe(b.ciphertext);
    });

    it("rejects ciphertext encrypted with a different key", async () => {
      const k1 = await importRoomKey(makeRandomKeyBase64());
      const k2 = await importRoomKey(makeRandomKeyBase64());
      const env = await seal(k1, "secret");
      await expect(open(k2, env)).rejects.toThrow();
    });

    it("rejects tampered ciphertext", async () => {
      const key = await importRoomKey(makeRandomKeyBase64());
      const env = await seal(key, "secret");

      // Flip a byte in the ciphertext (decode → mutate → re-encode).
      const bin = atob(env.ciphertext);
      const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
      bytes[0] = bytes[0] ^ 0x01;
      const tampered = btoa(String.fromCharCode(...bytes));

      await expect(open(key, { iv: env.iv, ciphertext: tampered })).rejects.toThrow();
    });

    it("rejects an envelope with the wrong IV size", async () => {
      const key = await importRoomKey(makeRandomKeyBase64());
      const env = await seal(key, "secret");
      // Replace IV with one of the wrong length
      const badIV = btoa("short"); // 5 bytes
      await expect(open(key, { iv: badIV, ciphertext: env.ciphertext })).rejects.toThrow(
        /Invalid IV length/,
      );
    });
  });
});
