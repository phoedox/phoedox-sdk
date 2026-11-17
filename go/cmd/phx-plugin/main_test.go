package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTheme(t *testing.T) string {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "phoedox-plugin.json"), []byte(`{"type":"theme","id":"de.example.forest","name":"Forest","version":"1.0.0","engines":{"phoedox":">=1.0.0"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "theme.json"), []byte(`{"light":{"accent":"#2c6e49"},"dark":{}}`), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "screenshots"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "screenshots", "one.png"), []byte("png"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not packed"), 0o644)
	return dir
}

func TestPackSignPublish(t *testing.T) {
	dir := writeTheme(t)
	out := t.TempDir()
	p, err := Pack(dir, out, true)
	if err != nil || filepath.Base(p) != "de.example.forest-1.0.0.phxp" {
		t.Fatalf("pack: %v %s", err, p)
	}
	data, _ := os.ReadFile(p)
	// Reproducible: packing again gives the same bytes.
	p2, _ := Pack(dir, t.TempDir(), true)
	if again, _ := os.ReadFile(p2); string(again) != string(data) {
		t.Fatal("package not reproducible")
	}
	m, err := manifestOf(data)
	if err != nil || m.ID != "de.example.forest" {
		t.Fatalf("manifest: %v", err)
	}
	// An invalid manifest is refused before anything is built.
	bad := t.TempDir()
	_ = os.WriteFile(filepath.Join(bad, "phoedox-plugin.json"), []byte(`{"id":"x","name":"X","version":"1"}`), 0o644)
	if _, err := Pack(bad, t.TempDir(), true); err == nil {
		t.Fatal("invalid manifest packed")
	}

	// Signature verifies against the public key.
	pub, priv, _ := ed25519.GenerateKey(nil)
	key := "k1:" + base64.StdEncoding.EncodeToString(priv.Seed())
	h, err := Sign(data, key)
	if err != nil || !strings.HasPrefix(h, "k1:") {
		t.Fatalf("sign: %v %s", err, h)
	}
	sig, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "k1:"))
	sum := sha256.Sum256(data)
	if !ed25519.Verify(pub, signedMessage("de.example.forest", "1.0.0", hex.EncodeToString(sum[:])), sig) {
		t.Fatal("signature does not verify")
	}

	// Publish: success and a store error with a hint.
	var gotAuth, gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotAuth, gotSig = r.Header.Get("Authorization"), r.Header.Get("Phoedox-Author-Signature")
		if r.URL.Query().Get("channel") == "beta" {
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"code":"version_exists","error":"this version exists already"}`))
			return
		}
		sum := sha256.Sum256(body)
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "de.example.forest", "version": "1.0.0", "sha256": hex.EncodeToString(sum[:]), "reviewState": "pending", "channel": "stable"})
	}))
	defer srv.Close()
	res, err := Publish(srv.Client(), srv.URL, data, "phxu_abc", h, false)
	if err != nil || res.ReviewState != "pending" || gotAuth != "Bearer phxu_abc" || gotSig != h {
		t.Fatalf("publish: %v %+v %s", err, res, gotAuth)
	}
	if _, err := Publish(srv.Client(), srv.URL, data, "phxu_abc", "", true); err == nil || !strings.Contains(err.Error(), "raise the version") {
		t.Fatalf("hint: %v", err)
	}
}

func TestGitHubIDToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "bearer req-token" || r.URL.Query().Get("audience") != "store.phoedox.com" {
			w.WriteHeader(403)
			return
		}
		_, _ = w.Write([]byte(`{"value":"eyJ.id.token"}`))
	}))
	defer srv.Close()
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", srv.URL+"/token?api-version=2.0")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "req-token")
	tok, err := githubIDToken(srv.Client(), "store.phoedox.com")
	if err != nil || tok != "eyJ.id.token" {
		t.Fatalf("id token: %v %s", err, tok)
	}
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	if _, err := githubIDToken(srv.Client(), "x"); err == nil {
		t.Fatal("outside GitHub Actions there is no ID token")
	}
}
