/**
 * Vitest setup: registers @testing-library/jest-dom matchers and any
 * polyfills needed by the modules we test under jsdom.
 */
import "@testing-library/jest-dom/vitest";

// jsdom doesn't ship a usable WebCrypto. cryptoService unit tests need
// real AES-GCM, so route window.crypto through Node's webcrypto when
// running tests. Browsers always have it; this only patches the test
// environment.
import { webcrypto } from "node:crypto";

if (typeof globalThis.crypto === "undefined" || !globalThis.crypto.subtle) {
  // @ts-expect-error - jsdom's crypto isn't fully typed as Crypto
  globalThis.crypto = webcrypto;
}
