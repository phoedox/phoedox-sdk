# Phoedox SDK

Tools for Phoedox plugins.

| Folder | Content |
|---|---|
| `go/pdk` | Go PDK for backend plugins (WASM, runs via Extism in the Phoedox server) |
| `js` | `@phoedox/sdk` – frontend SDK with Lit (`definePlugin`, `PhxElement`) |
| `schema` | JSON Schema for `phoedox-plugin.json` |

Official plugins are in [phoedox-plugins](https://github.com/phoedox/phoedox-plugins),
the host side is in [phoedox](https://github.com/phoedox/phoedox) (`internal/plugins`).

## Building a plugin

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o backend.wasm ./backend
```

## License

MIT – see [LICENSE](LICENSE).
