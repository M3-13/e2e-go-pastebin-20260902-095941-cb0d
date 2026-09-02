# Go Pastebin REST-API

Eine kleine Pastebin-REST-API in Go, ausschließlich mit der Standardbibliothek
(`net/http`). Paster können angelegt, abgerufen, aufgelistet und gelöscht
werden. Die Daten liegen in einem In-Memory-Store mit Mutex; ein optionaler
Ablauf über `expires_in_seconds` entfernt abgelaufene Paster automatisch.

## Tech-Stack

- **Sprache:** Go (1.22+)
- **Framework:** `net/http` (Standardbibliothek, keine externen Dependencies)
- **Storage:** In-Memory mit `sync.RWMutex`
- **Testing:** `go test` / `net/http/httptest`

## Ausführen

Voraussetzung ist eine Go-Installation (1.22 oder neuer). Der Server liest die
Umgebungsvariable `PORT` (Standard: `8080`) und startet:

```sh
go run .
```

Anschließend antwortet der Server auf `http://localhost:8080`.

### Build für Produktion

```sh
go build ./...
```

## Endpunkte

| Methode | Pfad          | Beschreibung                                  |
| ------- | ------------- | --------------------------------------------- |
| GET     | `/health`     | Health-Check, antwortet `200` mit `{"status":"ok"}` |
| POST    | `/pastes`     | Legt einen Paste an (`201`)                    |
| GET     | `/pastes/{id}`| Ruft einen Paste inklusive `content` ab (`200`) |
| GET     | `/pastes`     | Listet die Metadaten aller Paster ohne `content` |
| DELETE  | `/pastes/{id}`| Löscht einen Paste (`204`)                     |

Fehlerantworten (Statuscode >= 400) liefern ausschließlich ein JSON-Objekt mit
einem `error`-Feld, z. B.:

```json
{ "error": "not found" }
```

Nicht unterstützte Methoden auf `/pastes` antworten mit `405`, unbekannte
Pfade mit `404` – jeweils als JSON-Fehler.

## Umgebungsvariablen

| Variable | Bedeutung                     | Standard |
| -------- | ----------------------------- | -------- |
| `PORT`   | Port, auf dem der Server lauscht | `8080` |
