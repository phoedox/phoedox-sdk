package main

import (
	"archive/tar"
	"compress/gzip"
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
	"slices"
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

	// Logos are packed after the SVG check; a logo with a script is refused.
	logo := t.TempDir()
	_ = os.WriteFile(filepath.Join(logo, "phoedox-plugin.json"), []byte(`{"type":"theme","id":"de.example.logo","name":"Logo","version":"1.0.0","engines":{"phoedox":">=1.0.0"},"category":"home","logo":"logo.svg"}`), 0o644)
	_ = os.WriteFile(filepath.Join(logo, "logo.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="4"/></svg>`), 0o644)
	p3, err := Pack(logo, t.TempDir(), true)
	if err != nil {
		t.Fatalf("logo: %v", err)
	}
	if names := packageNames(t, p3); !slices.Contains(names, "logo.svg") {
		t.Fatalf("logo not in the package: %v", names)
	}
	_ = os.WriteFile(filepath.Join(logo, "logo.svg"), []byte(`<svg onload="alert(1)"/>`), 0o644)
	if _, err := Pack(logo, t.TempDir(), true); err == nil || !strings.Contains(err.Error(), "event handler") {
		t.Fatalf("unsafe logo packed: %v", err)
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
	var gotAuth, gotSig, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotAuth, gotSig, gotQuery = r.Header.Get("Authorization"), r.Header.Get("Phoedox-Author-Signature"), r.URL.RawQuery
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
	if _, err := Publish(srv.Client(), srv.URL, data, "phxu_abc", h, false, true); err != nil || gotQuery != "logoRights=true" {
		t.Fatalf("logo rights: %v %q", err, gotQuery)
	}
	if got := reorder([]string{"pkg.phxp", "-logo-rights", "-store", "x"}); strings.Join(got, " ") != "-logo-rights -store x pkg.phxp" {
		t.Fatalf("reorder: %v", got)
	}
	res, err := Publish(srv.Client(), srv.URL, data, "phxu_abc", h, false, false)
	if err != nil || res.ReviewState != "pending" || gotAuth != "Bearer phxu_abc" || gotSig != h {
		t.Fatalf("publish: %v %+v %s", err, res, gotAuth)
	}
	if _, err := Publish(srv.Client(), srv.URL, data, "phxu_abc", "", true, false); err == nil || !strings.Contains(err.Error(), "raise the version") {
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

func packageNames(t *testing.T, p string) []string {
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	tr := tar.NewReader(zr)
	for h, err := tr.Next(); err == nil; h, err = tr.Next() {
		names = append(names, h.Name)
	}
	return names
}
