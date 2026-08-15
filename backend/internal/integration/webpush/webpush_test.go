package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// receiver mirrors the user-agent half of RFC 8291: it holds the key pair a
// browser would have minted, and decrypts what the client produces.
type receiver struct {
	private *ecdh.PrivateKey
	auth    []byte
}

func newReceiver(t *testing.T) receiver {
	t.Helper()
	private, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate subscription key: %v", err)
	}
	auth := make([]byte, authLength)
	if _, err := io.ReadFull(rand.Reader, auth); err != nil {
		t.Fatalf("generate auth secret: %v", err)
	}
	return receiver{private: private, auth: auth}
}

func (r receiver) subscription(endpoint string) Subscription {
	return Subscription{
		Endpoint: endpoint,
		P256dh:   b64.EncodeToString(r.private.PublicKey().Bytes()),
		Auth:     b64.EncodeToString(r.auth),
	}
}

// open parses the aes128gcm body and recovers the plaintext, following the
// receiver steps of RFC 8188 §2 and the key schedule of RFC 8291 §3.
func (r receiver) open(body []byte) ([]byte, error) {
	if len(body) < headerLength {
		return nil, errors.New("body shorter than the aes128gcm header")
	}
	salt := body[:saltLength]
	if size := binary.BigEndian.Uint32(body[saltLength : saltLength+4]); size != recordSize {
		return nil, errors.New("unexpected record size")
	}
	if idLength := int(body[saltLength+4]); idLength != publicKeyLength {
		return nil, errors.New("unexpected key id length")
	}
	serverPublic := body[saltLength+5 : headerLength]

	senderKey, err := ecdh.P256().NewPublicKey(serverPublic)
	if err != nil {
		return nil, err
	}
	shared, err := r.private.ECDH(senderKey)
	if err != nil {
		return nil, err
	}

	keyInfo := append([]byte("WebPush: info\x00"), r.private.PublicKey().Bytes()...)
	keyInfo = append(keyInfo, serverPublic...)
	ikm, err := hkdf.Key(sha256.New, shared, r.auth, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	key, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	record, err := aead.Open(nil, nonce, body[headerLength:], nil)
	if err != nil {
		return nil, err
	}
	// Strip the RFC 8188 padding delimiter; ours is always a final record.
	if len(record) == 0 || record[len(record)-1] != 0x02 {
		return nil, errors.New("missing final-record delimiter")
	}
	return record[:len(record)-1], nil
}

func TestEncryptRoundTripsThroughAReceiver(t *testing.T) {
	ua := newReceiver(t)
	plaintext := []byte(`{"kind":"question","chatId":"beefcafe"}`)

	body, err := encrypt(plaintext, ua.private.PublicKey().Bytes(), ua.auth, nil)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if want := headerLength + len(plaintext) + 1 + 16; len(body) != want {
		t.Fatalf("body length = %d, want %d", len(body), want)
	}
	if got := binary.BigEndian.Uint32(body[saltLength : saltLength+4]); got != recordSize {
		t.Fatalf("record size header = %d, want %d", got, recordSize)
	}

	got, err := ua.open(body)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext = %q, want %q", got, plaintext)
	}
}

func TestEncryptUsesAFreshSaltAndEphemeralKeyPerMessage(t *testing.T) {
	ua := newReceiver(t)
	first, err := encrypt([]byte("same"), ua.private.PublicKey().Bytes(), ua.auth, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := encrypt([]byte("same"), ua.private.PublicKey().Bytes(), ua.auth, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first[:headerLength], second[:headerLength]) {
		t.Fatal("two messages reused the same salt and ephemeral key")
	}
}

func TestEncryptRejectsOversizedPayloads(t *testing.T) {
	ua := newReceiver(t)
	_, err := encrypt(bytes.Repeat([]byte("x"), maxPlaintext+1), ua.private.PublicKey().Bytes(), ua.auth, nil)
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("err = %v, want ErrPayloadTooLarge", err)
	}
}

func TestEncryptRejectsAMalformedAuthSecret(t *testing.T) {
	ua := newReceiver(t)
	if _, err := encrypt([]byte("hi"), ua.private.PublicKey().Bytes(), ua.auth[:8], nil); err == nil {
		t.Fatal("expected a short auth secret to be rejected")
	}
}

func TestVAPIDKeyRoundTripsThroughItsEncodedForm(t *testing.T) {
	key, err := GenerateVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ParseVAPIDKey(key.PrivateKeyBase64())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if restored.PublicKeyBase64() != key.PublicKeyBase64() {
		t.Fatal("public key did not survive the round trip")
	}
	if raw, err := decodeBase64(key.PublicKeyBase64()); err != nil || len(raw) != publicKeyLength {
		t.Fatalf("public key = %d bytes (err %v), want %d", len(raw), err, publicKeyLength)
	}
}

func TestAuthorizationHeaderCarriesASignedTokenForTheEndpointOrigin(t *testing.T) {
	key, err := GenerateVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)

	header, err := key.authorization("https://fcm.googleapis.com/fcm/send/abc123", "https://remote.example.com", now)
	if err != nil {
		t.Fatalf("authorization: %v", err)
	}
	token, ok := strings.CutPrefix(header, "vapid t=")
	if !ok {
		t.Fatalf("header = %q, want a vapid scheme", header)
	}
	token, publicKey, ok := strings.Cut(token, ", k=")
	if !ok {
		t.Fatalf("header = %q, want a k= parameter", header)
	}
	if publicKey != key.PublicKeyBase64() {
		t.Fatal("header advertised a different public key")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}
	claims := map[string]any{}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("parse claims: %v", err)
	}
	// The audience is the endpoint's origin only — never its path, which
	// would leak the subscription id to anyone who sees the token.
	if claims["aud"] != "https://fcm.googleapis.com" {
		t.Fatalf("aud = %v, want the endpoint origin", claims["aud"])
	}
	if claims["sub"] != "https://remote.example.com" {
		t.Fatalf("sub = %v", claims["sub"])
	}
	if exp, _ := claims["exp"].(float64); int64(exp) != now.Add(vapidTokenLifetime).Unix() {
		t.Fatalf("exp = %v, want %d", claims["exp"], now.Add(vapidTokenLifetime).Unix())
	}
}

func TestNormalizeSubject(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "https://remote.example.com/", want: "https://remote.example.com"},
		{in: "mailto:ops@example.com", want: "mailto:ops@example.com"},
		{in: "ops@example.com", want: "mailto:ops@example.com"},
		{in: "", wantErr: true},
		{in: "http://remote.example.com", wantErr: true},
	} {
		got, err := NormalizeSubject(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("NormalizeSubject(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeSubject(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestSendPostsAnEncryptedBodyTheSubscriptionCanOpen(t *testing.T) {
	ua := newReceiver(t)
	payload := []byte(`{"title":"Claude is asking"}`)

	var opened []byte
	var headers http.Header
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		var err error
		if opened, err = ua.open(body); err != nil {
			t.Errorf("receiver could not open the body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	key, err := GenerateVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(key, "ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	client.http = server.Client()

	err = client.Send(
		context.Background(),
		ua.subscription(server.URL+"/push/abc"),
		payload,
		Options{TTL: 90 * time.Second, Urgency: UrgencyHigh, Topic: "chat-beefcafe"},
	)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !bytes.Equal(opened, payload) {
		t.Fatalf("delivered payload = %q, want %q", opened, payload)
	}
	if got := headers.Get("Content-Encoding"); got != "aes128gcm" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if got := headers.Get("TTL"); got != "90" {
		t.Fatalf("TTL = %q, want 90", got)
	}
	if got := headers.Get("Urgency"); got != "high" {
		t.Fatalf("Urgency = %q", got)
	}
	if got := headers.Get("Topic"); got != "chat-beefcafe" {
		t.Fatalf("Topic = %q", got)
	}
	if !strings.HasPrefix(headers.Get("Authorization"), "vapid t=") {
		t.Fatalf("Authorization = %q", headers.Get("Authorization"))
	}
}

func TestSendReportsRetiredSubscriptionsAsGone(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone} {
		ua := newReceiver(t)
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		key, err := GenerateVAPIDKey()
		if err != nil {
			t.Fatal(err)
		}
		client, err := NewClient(key, "ops@example.com")
		if err != nil {
			t.Fatal(err)
		}
		client.http = server.Client()

		err = client.Send(context.Background(), ua.subscription(server.URL+"/push/abc"), []byte("{}"), Options{})
		if !errors.Is(err, ErrSubscriptionGone) {
			t.Fatalf("status %d: err = %v, want ErrSubscriptionGone", status, err)
		}
		server.Close()
	}
}

func TestSendSurfacesOtherFailures(t *testing.T) {
	ua := newReceiver(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer server.Close()

	key, err := GenerateVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(key, "ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	client.http = server.Client()

	err = client.Send(context.Background(), ua.subscription(server.URL+"/push/abc"), []byte("{}"), Options{})
	if err == nil || errors.Is(err, ErrSubscriptionGone) {
		t.Fatalf("err = %v, want a plain failure", err)
	}
	if !strings.Contains(err.Error(), "slow down") {
		t.Fatalf("err = %v, want the push service's detail quoted", err)
	}
}

func TestSendRejectsNonHTTPSEndpoints(t *testing.T) {
	key, err := GenerateVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(key, "ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	ua := newReceiver(t)
	if err := client.Send(context.Background(), ua.subscription("http://push.example.com/x"), []byte("{}"), Options{}); err == nil {
		t.Fatal("expected an http:// endpoint to be rejected")
	}
}
