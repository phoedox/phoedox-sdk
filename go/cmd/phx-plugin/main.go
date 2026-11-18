// Command phx-plugin packs, signs and publishes Phoedox plugins, themes and
// app templates (ADR-0074):
//
//	phx-plugin pack [dir] [-o dist]          build backend/frontend, check the manifest, write <id>-<version>.phxp
//	phx-plugin keygen [-id KEY_ID]            new ed25519 author key (register the public key in the store)
//	phx-plugin sign <package>                 print the Phoedox-Author-Signature value (key in PHOEDOX_AUTHOR_KEY)
//	phx-plugin publish <package> [-beta]      upload for review
//
// publish authenticates with PHOEDOX_STORE_TOKEN (author or upload token)
// or, inside GitHub Actions with "permissions: id-token: write", with the
// workflow's ID token (trusted publishing). Uploads always go into the
// store's review queue.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/phoedox/phoedox-sdk/go/pdk"
)

const usage = `phx-plugin – pack, sign and publish Phoedox plugins, themes and app templates

  phx-plugin pack [dir] [-o dist]        build and check, write dist/<id>-<version>.phxp
  phx-plugin keygen [-id KEY_ID]         create an ed25519 author key
  phx-plugin sign <package>              print the author signature (key in PHOEDOX_AUTHOR_KEY)
  phx-plugin publish <package> [-beta] [-store URL]
                                         upload for review (PHOEDOX_STORE_TOKEN, or the GitHub
                                         Actions ID token with permissions: id-token: write)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "pack":
		err = cmdPack(os.Args[2:])
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "sign":
		err = cmdSign(os.Args[2:])
	case "publish":
		err = cmdPublish(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "phx-plugin:", err)
		os.Exit(1)
	}
}

// --- pack -----------------------------------------------------------------

// copied are the files and folders of a plugin folder that go into the
// package as they are (built backend and frontend are added separately).
var copied = []string{"schemas", "screenshots", "files", "theme.json", "theme.css", "app.json", "LICENSE", "README.md"}

func cmdPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	out := fs.String("o", "dist", "output folder")
	_ = fs.Parse(reorder(args))
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	p, err := Pack(dir, *out, true)
	if err != nil {
		return err
	}
	fmt.Println(p)
	return nil
}

// Pack builds a package from a plugin folder and returns its path. With
// build, the backend (Go → WASM reactor) and frontend (Bun) are built.
func Pack(dir, outDir string, build bool) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "phoedox-plugin.json"))
	if err != nil {
		return "", err
	}
	m, err := pdk.ParseManifest(raw)
	if err != nil {
		return "", err
	}
	files := map[string][]byte{"phoedox-plugin.json": raw}
	if m.Backend != nil {
		name := path.Clean(m.Backend.Wasm)
		if build {
			tmp, err := os.MkdirTemp("", "phx-wasm-*")
			if err != nil {
				return "", err
			}
			defer os.RemoveAll(tmp)
			target := filepath.Join(tmp, "backend.wasm")
			cmd := exec.Command("go", "build", "-buildmode=c-shared", "-trimpath", "-o", target, "./backend")
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stderr, os.Stderr
			cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("build backend: %w", err)
			}
			if files[name], err = os.ReadFile(target); err != nil {
				return "", err
			}
		} else if files[name], err = os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			return "", fmt.Errorf("backend %s: %w", name, err)
		}
	}
	if m.Frontend != nil {
		name := path.Clean(m.Frontend.Module)
		src := filepath.Join(dir, "frontend", "index.ts")
		if _, err := os.Stat(src); err == nil && build {
			tmp, err := os.MkdirTemp("", "phx-js-*")
			if err != nil {
				return "", err
			}
			defer os.RemoveAll(tmp)
			target := filepath.Join(tmp, "index.js")
			cmd := exec.Command("bun", "build", src, "--format", "esm", "--minify", "--outfile", target)
			cmd.Stdout, cmd.Stderr = io.Discard, os.Stderr
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("build frontend: %w", err)
			}
			if files[name], err = os.ReadFile(target); err != nil {
				return "", err
			}
		} else if files[name], err = os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			return "", fmt.Errorf("frontend %s: %w", name, err)
		}
	}
	for _, c := range copied {
		full := filepath.Join(dir, c)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if files[c], err = os.ReadFile(full); err != nil {
				return "", err
			}
			continue
		}
		err = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				return err
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)], err = os.ReadFile(p)
			return err
		})
		if err != nil {
			return "", err
		}
	}
	for _, l := range []string{m.Logo, m.LogoDark} {
		if l == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, l))
		if err != nil {
			return "", fmt.Errorf("logo %s: %w", l, err)
		}
		if err := pdk.CheckSVG(b); err != nil {
			return "", fmt.Errorf("%s: %w", l, err)
		}
		files[l] = b
	}
	data, err := tarGz(files)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(outDir, m.ID+"-"+m.Version+".phxp")
	return out, os.WriteFile(out, data, 0o644)
}

// tarGz writes a reproducible package: sorted names, no times or owners.
func tarGz(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, n := range names {
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(files[n])), Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0)}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(files[n]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// manifestOf reads the manifest of a package.
func manifestOf(data []byte) (pdk.Manifest, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return pdk.Manifest{}, fmt.Errorf("not a package: %w", err)
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			return pdk.Manifest{}, errors.New("phoedox-plugin.json is missing")
		}
		if strings.TrimPrefix(h.Name, "./") == "phoedox-plugin.json" {
			raw, err := io.ReadAll(io.LimitReader(tr, 1<<20))
			if err != nil {
				return pdk.Manifest{}, err
			}
			return pdk.ParseManifest(raw)
		}
	}
}

// --- keys and signatures ----------------------------------------------------

// signedMessage is what the author signs (the same as the store and the
// Phoedox server: id, version and SHA-256 of the package).
func signedMessage(id, version, sha256hex string) []byte {
	return []byte("phoedox-plugin-v1\n" + id + "\n" + version + "\n" + sha256hex)
}

// parseKey reads "keyId:base64(seed or private key)".
func parseKey(s string) (string, ed25519.PrivateKey, error) {
	id, b64, ok := strings.Cut(strings.TrimSpace(s), ":")
	raw, err := base64.StdEncoding.DecodeString(b64)
	if !ok || id == "" || err != nil {
		return "", nil, errors.New("PHOEDOX_AUTHOR_KEY must be keyId:base64(key)")
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return id, ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return id, ed25519.PrivateKey(raw), nil
	}
	return "", nil, errors.New("PHOEDOX_AUTHOR_KEY has the wrong length")
}

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	id := fs.String("id", "", "key id (default: date)")
	_ = fs.Parse(args)
	if *id == "" {
		*id = "author-" + time.Now().UTC().Format("20060102")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Printf("Private key (keep secret, e.g. PHOEDOX_AUTHOR_KEY):\n%s:%s\n\nPublic key (register it in the store's author area with key id %s):\n%s\n",
		*id, base64.StdEncoding.EncodeToString(priv.Seed()), *id, base64.StdEncoding.EncodeToString(pub))
	return nil
}

// Sign returns the Phoedox-Author-Signature value of a package.
func Sign(data []byte, key string) (string, error) {
	m, err := manifestOf(data)
	if err != nil {
		return "", err
	}
	id, priv, err := parseKey(key)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	sig := ed25519.Sign(priv, signedMessage(m.ID, m.Version, hex.EncodeToString(sum[:])))
	return id + ":" + base64.StdEncoding.EncodeToString(sig), nil
}

func cmdSign(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: phx-plugin sign <package>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	h, err := Sign(data, os.Getenv("PHOEDOX_AUTHOR_KEY"))
	if err != nil {
		return err
	}
	fmt.Println(h)
	return nil
}

// --- publish ----------------------------------------------------------------

// Result is the store's answer to an upload.
type Result struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	SHA256      string `json:"sha256"`
	ReviewState string `json:"reviewState"`
	Channel     string `json:"channel"`
	Flags       []struct {
		Level string `json:"level"`
		Key   string `json:"key"`
	} `json:"flags"`
}

// githubIDToken asks GitHub Actions for an ID token with the audience.
func githubIDToken(hc *http.Client, audience string) (string, error) {
	reqURL, reqTok := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if reqURL == "" || reqTok == "" {
		return "", errors.New("no store token: set PHOEDOX_STORE_TOKEN, or run in GitHub Actions with permissions: id-token: write")
	}
	u, err := url.Parse(reqURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("Authorization", "bearer "+reqTok)
	res, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		Value string `json:"value"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(res.Body).Decode(&out) != nil || out.Value == "" {
		return "", fmt.Errorf("GitHub did not give an ID token (HTTP %d)", res.StatusCode)
	}
	return out.Value, nil
}

// hints explain the store's error codes.
var hints = map[string]string{
	"version_exists":   "this version is in the store already – raise the version in phoedox-plugin.json",
	"not_owner":        "this id belongs to another author",
	"author_signature": "set PHOEDOX_AUTHOR_KEY to sign, or register your public key in the author area",
	"not_trusted":      "register this repository and workflow as trusted publisher of the plugin in the author area",
	"token_scope":      "this upload token belongs to another plugin",
	"oidc":             "the GitHub ID token was refused – check the audience (-audience) and the workflow permissions",
	"unauthorized":     "the token is unknown, expired or revoked",
}

// Publish uploads a package and returns the store's answer.
func Publish(hc *http.Client, store string, data []byte, token, signature string, beta bool) (Result, error) {
	var res Result
	u := strings.TrimRight(store, "/") + "/api/v1/plugins"
	if beta {
		u += "?channel=beta"
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return res, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "phx-plugin")
	if signature != "" {
		req.Header.Set("Phoedox-Author-Signature", signature)
	}
	r, err := hc.Do(req)
	if err != nil {
		return res, err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if r.StatusCode != http.StatusCreated && r.StatusCode != http.StatusOK {
		var e struct {
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		msg := fmt.Sprintf("store answered %d", r.StatusCode)
		if e.Code != "" || e.Error != "" {
			msg = fmt.Sprintf("store: %s (%s)", e.Error, e.Code)
		}
		if h := hints[e.Code]; h != "" {
			msg += "\n  → " + h
		}
		return res, errors.New(msg)
	}
	return res, json.Unmarshal(body, &res)
}

func cmdPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	store := fs.String("store", envOr("PHOEDOX_STORE_URL", "https://store.phoedox.com"), "store address")
	beta := fs.Bool("beta", false, "publish to the beta channel")
	audience := fs.String("audience", "", "audience of the GitHub ID token (default: the store's host)")
	_ = fs.Parse(reorder(args))
	if fs.NArg() != 1 {
		return errors.New("usage: phx-plugin publish <package> [-beta] [-store URL]")
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	if _, err := manifestOf(data); err != nil {
		return err
	}
	hc := &http.Client{Timeout: 2 * time.Minute}
	token, via := os.Getenv("PHOEDOX_STORE_TOKEN"), "token"
	if token == "" {
		aud := *audience
		if aud == "" {
			su, err := url.Parse(*store)
			if err != nil {
				return err
			}
			aud = su.Hostname()
		}
		if token, err = githubIDToken(hc, aud); err != nil {
			return err
		}
		via = "trusted publishing"
	}
	sig := ""
	if k := os.Getenv("PHOEDOX_AUTHOR_KEY"); k != "" {
		if sig, err = Sign(data, k); err != nil {
			return err
		}
	}
	res, err := Publish(hc, *store, data, token, sig, *beta)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s uploaded via %s (%s channel), review state: %s\nsha256 %s\n%s/author/plugins\n",
		res.ID, res.Version, via, res.Channel, res.ReviewState, res.SHA256, strings.TrimRight(*store, "/"))
	if f := os.Getenv("GITHUB_STEP_SUMMARY"); f != "" {
		out, err := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(out, "### Phoedox store upload\n\n| | |\n|---|---|\n| Item | `%s` |\n| Version | `%s` |\n| Channel | %s |\n| Review | %s |\n| SHA-256 | `%s` |\n| Signed | %v |\n\nThe version waits for a moderator before it is published.\n",
				res.ID, res.Version, res.Channel, res.ReviewState, res.SHA256, sig != "")
			out.Close()
		}
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// reorder puts flags before positional arguments (pack dir -o out).
func reorder(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && a != "-beta" && a != "--beta" && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}
