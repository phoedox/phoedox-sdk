# Phoedox SDK

Plugin-SDK für Phoedox. Gesamtentscheidungen: in Phoedox (https://app.phoedox.com, Projekt `phoedox`), v. a. ADR-0003 und ADR-0024; per MCP lesen, neue mit `create_decision` anlegen.

## Regeln

- **Dokumentation und Aufgaben auf Englisch:** README, `docs/`, Phoedox-Seiten, Entscheidungen (ADRs) und Karten auf dem Board „Roadmap“ immer auf Englisch schreiben; Deutsch nur als Übersetzung (Projekt `phoedox` hat Englisch als Hauptsprache).
- Immer die aktuelle stabile Go-Version und aktuelle Abhängigkeiten verwenden (`go` + `toolchain` in `go.mod`).
- `schema/phoedox-plugin.schema.json`, `go/pdk/manifest.go` und `js/src/index.ts` beschreiben dasselbe Manifest – immer gemeinsam ändern.
- `pdk.HostAPI` ist die Hauptversion der Host-API. Neue Host-Funktionen ohne Bruch; Breaking Changes nur mit neuer Hauptversion (Host unterstützt aktuelle + vorige).
- Rechte im Manifest sind eine Whitelist (`permRe` in `manifest.go`). Neue Rechte zuerst im Host implementieren, dann hier freischalten.
- Plugin-Elemente heißen `phx-<name>`; Icons sind Lucide-Namen.
- Go: `go test ./...` und `GOOS=wasip1 GOARCH=wasm go build ./...` in `go/`. JS: `bun test` und `bun run build` in `js/`.
- Commits nach Conventional Commits mit Scope `go` oder `js` (Release-Please, getrennte Releases).
- Lizenz MIT. Code und Kommentare Englisch.
