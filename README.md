# Phoedox SDK

Tools for Phoedox plugins.

| Folder | Content |
|---|---|
| `go/pdk` | Go PDK for backend plugins (WASM, runs via Extism in the Phoedox server) |
| `js` | `@phoedox/sdk` – frontend SDK with Lit (`definePlugin`, `PhxElement`) |
| `schema` | JSON Schema for `phoedox-plugin.json` |
| `go/cmd/phx-plugin` | command line: pack, sign and publish plugins, themes and app templates |
| `actions/publish` | GitHub Action that packs and submits to the store |

Official plugins are in [phoedox-plugins](https://github.com/phoedox/phoedox-plugins),
the host side is in [phoedox](https://github.com/phoedox/phoedox) (`internal/plugins`).

## Building a plugin

```bash
go install github.com/phoedox/phoedox-sdk/go/cmd/phx-plugin@latest
phx-plugin pack plugins/ups -o dist      # checks the manifest, builds backend (Go → WASM reactor) and frontend (Bun)
```

`pack` writes a reproducible `dist/<id>-<version>.phxp` with the manifest, the built `backend.wasm`
and frontend module, and `schemas/`, `screenshots/`, `files/`, `theme.json`, `theme.css`, `app.json`
when they exist. Without the CLI: `GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o backend.wasm ./backend`.

## Content blocks (ADR-0086)

A plugin can add a block kind for pages: a fenced block whose info string is the block's id
(```` ```gantt ````). Declare it in the manifest:

```json
"contributes": { "blocks": [{ "id": "gantt", "label": "Schedule", "icon": "chart-gantt",
  "element": "phx-gantt", "board": true, "example": "Task: 2026-03-01, 2w" }] }
```

The backend draws it – it must export `block_render`:

```go
//go:wasmexport block_render
func blockRender() int32 { return pdk.HandleBlockRender(render) }

func render(in pdk.BlockRender) (pdk.BlockResult, error) { … } // SVG + problems
```

The SVG is shown everywhere (web app, mobile app, published pages, whiteboards; the host
removes scripts, event handlers and outside links) and the problems check AI writes. Use
the CSS variables `--fg`, `--muted`, `--line`, `--surface`, `--tc-<colour>` and
`--tb-<colour>` so the picture follows light and dark mode. The optional `element` of the
frontend makes the block interactive in the web app and, in a WebView, in the mobile app;
the host sets its `source`, `svg`, `project`, `doc` and `stateKey` properties.

**State per reader** (votes, answers, ticked steps): the element keeps it with
`sdk.blocks.state({ doc, key: stateKey }).get()` / `.put(data)` – a JSON object up to 16 KB per
reader, page and block. A state `{ "choices": [...] }` is counted for all readers (`votes`,
`voters`; never who). `stateKey` is the whiteboard element's id or a hash of the trimmed source,
the same keys the built-in blocks used, so state survives a block's move into a plugin; a
plugin may use its own `id:` from the source instead.

**On whiteboards:** `"board": true` lets people place the block, `"size": { "w": 520, "h": 260 }`
sets its first size, and `"live": true` runs the element there instead of only the picture
(polls, for example).

## Publishing to the store

Every upload goes into the store's review queue; nothing is published without a moderator (ADR-0074).

**From GitHub Actions (trusted publishing, no secret):** register the repository and workflow file as
trusted publisher of the plugin in the store's author area (page *Command line*), then:

```yaml
on:
  push:
    tags: ["v*"]
permissions:
  contents: read
  id-token: write          # the workflow gets a GitHub ID token for the store
jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: stable }
      - uses: oven-sh/setup-bun@v2   # only for plugins with a frontend
      - uses: phoedox/phoedox-sdk/actions/publish@main
        with:
          path: .
```

**Other CI:** create an upload token (one plugin, upload only, with an expiry date) in the author area and
set it as `PHOEDOX_STORE_TOKEN` (or the action's `upload-token`):

```bash
PHOEDOX_STORE_TOKEN=phxu_… phx-plugin publish dist/de.example.ups-1.2.0.phxp [-beta]
```

The author token (`phxa_…`) still works, but it can do everything your account can; moderators see which
way a version came in.

**Author signature (optional, required if the store asks for it):** `phx-plugin keygen` creates a key pair;
register the public key in the author area and keep the private key in `PHOEDOX_AUTHOR_KEY`
(`keyId:base64`). `publish` signs automatically when it is set; `phx-plugin sign <package>` prints the
`Phoedox-Author-Signature` value. Put the private key into CI only if you accept that whoever controls the
CI can sign as you.

## License

MIT – see [LICENSE](LICENSE).
