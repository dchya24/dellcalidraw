package crypto

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateKeyAndEncodeRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if len(key) != KeySize {
		t.Fatalf("key length: got %d, want %d", len(key), KeySize)
	}

	encoded := EncodeKey(key)
	decoded, err := DecodeKey(encoded)
	if err != nil {
		t.Fatalf("DecodeKey: %v", err)
	}
	if string(decoded) != string(key) {
		t.Fatalf("decoded key does not match original")
	}
}

func TestDecodeKeyRejectsWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("only-16-bytes!!!"))
	_, err := DecodeKey(short)
	if err == nil {
		t.Fatal("expected error for short key")
	}
	if !strings.Contains(err.Error(), "invalid key length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeKeyRejectsBadBase64(t *testing.T) {
	_, err := DecodeKey("not base64!!!")
	if err == nil {
		t.Fatal("expected error for malformed base64")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	key, _ := GenerateKey()

	plain := []byte(`{"type":"update_elements","payload":{"roomId":"r1"}}`)
	env, err := Seal(key, plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Wire format sanity: must be JSON-marshalable as the EncryptedEnvelope
	// shape the WebSocket protocol expects.
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("envelope is not JSON-serialisable: %v", err)
	}
	var parsed EncryptedEnvelope
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("envelope round-trip via JSON failed: %v", err)
	}

	got, err := Open(key, &parsed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("plaintext mismatch:\n got: %s\nwant: %s", got, plain)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	key, _ := GenerateKey()
	plain := []byte("hello")

	a, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}

	if a.IV == b.IV {
		t.Fatal("nonce must be unique per Seal call")
	}
	if a.Ciphertext == b.Ciphertext {
		t.Fatal("ciphertext must differ when nonce changes")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	k1, _ := GenerateKey()
	k2, _ := GenerateKey()

	env, err := Seal(k1, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(k2, env); err == nil {
		t.Fatal("expected error opening with wrong key")
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	env, err := Seal(key, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	// Flip a byte in the ciphertext
	ct, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	ct[0] ^= 0x01
	env.Ciphertext = base64.StdEncoding.EncodeToString(ct)

	if _, err := Open(key, env); err == nil {
		t.Fatal("expected error opening tampered ciphertext")
	}
}

func TestOpenRejectsBadIV(t *testing.T) {
	key, _ := GenerateKey()
	env, err := Seal(key, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	env.IV = base64.StdEncoding.EncodeToString([]byte("short"))

	if _, err := Open(key, env); err == nil {
		t.Fatal("expected error opening with bad IV")
	}
}

func TestOpenRejectsNilEnvelope(t *testing.T) {
	key, _ := GenerateKey()
	if _, err := Open(key, nil); err == nil {
		t.Fatal("expected error opening nil envelope")
	}
}

func TestSealRejectsBadKeyLength(t *testing.T) {
	if _, err := Seal([]byte("too short"), []byte("data")); err == nil {
		t.Fatal("expected error sealing with short key")
	}
}

// Cross-impl compatibility check: an envelope produced here must
// decode the same way the FE cryptoService expects (base64 IV +
// ciphertext, AES-GCM, 12-byte nonce). We can't run the FE here,
// but we can assert the wire shape.
func TestEnvelopeShapeMatchesProtocol(t *testing.T) {
	key, _ := GenerateKey()
	env, err := Seal(key, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	iv, err := base64.StdEncoding.DecodeString(env.IV)
	if err != nil {
		t.Fatalf("iv must be base64: %v", err)
	}
	if len(iv) != NonceSize {
		t.Fatalf("nonce size: got %d want %d", len(iv), NonceSize)
	}
	if _, err := base64.StdEncoding.DecodeString(env.Ciphertext); err != nil {
		t.Fatalf("ciphertext must be base64: %v", err)
	}
}
