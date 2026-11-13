# Phoedox SDK

Werkzeuge für Phoedox-Plugins.

| Ordner | Inhalt |
|---|---|
| `go/pdk` | Go-PDK für Backend-Plugins (WASM, läuft über Extism im Phoedox-Server) |
| `js` | `@phoedox/sdk` – Frontend-SDK mit Lit (`definePlugin`, `PhxElement`) |
| `schema` | JSON-Schema für `phoedox-plugin.json` |

Offizielle Plugins liegen in [phoedox-plugins](https://github.com/phoedox/phoedox-plugins),
die Host-Seite in [phoedox](https://github.com/phoedox/phoedox) (`internal/plugins`).

## Ein Plugin bauen

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o backend.wasm ./backend
```

## Lizenz

MIT – siehe [LICENSE](LICENSE).
