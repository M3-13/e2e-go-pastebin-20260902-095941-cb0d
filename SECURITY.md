VERDICT: CHANGES_REQUESTED

Sicherheitsprüfung des vollständig zusammengeführten Go-Backends (Pastebin-REST-API). Keine anwendbaren Security-Scanner-Ergebnisse vorhanden; die Bewertung basiert auf manueller Codeanalyse.

## Zusammenfassung der Prüfbereiche

- **Secrets:** Keine hartkodierten Passwörter, Tokens oder Schlüssel gefunden. `log.Fatal` in `main.go` gibt ausschließlich Serverstartfehler aus, keine Request-Inhalte oder Metadaten (AC-13/AC-14 eingehalten).
- **Injection & Inputs:** JSON-Validierung, `http.MaxBytesReader` (1 MiB), Ablehnung von `/` in Paste-IDs und keine SQL-/Command-Injection vorhanden. Kein HTML-Rendering, daher kein direktes XSS-Risiko.
- **AuthN/AuthZ:** Keine Authentifizierung oder Autorisierung implementiert. Die zufällige 128-Bit-ID wirkt faktisch als „Capability“, schützt aber nicht vor unbefugtem Löschen, sobald eine ID geteilt wurde.
- **Dependencies:** Nur Go-Standardbibliothek. Keine externen Pakete; kein Audit nötig.
- **Configuration/Transport:** Keine Server-Timeouts, kein TLS, keine Raten-/Speicherlimits. Diese Punkte sind als Härtungslücken einzustufen.

## Findings

### 1. Fehlende HTTP-Server-Timeouts (Medium)
- **Betroffene Stelle:** `main.go`, Zeile `server := &http.Server{...}`
- **Beschreibung:** Der Server startet ohne `ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout` oder `IdleTimeout`. Ein Angreifer kann langsame Verbindungen (Slowloris) aufbauen und Serverressourcen binden, was zu einem Denial-of-Service führen kann.
- **Konkreter Fix:**
  ```go
  import "time"

  server := &http.Server{
      Addr:              ":" + port,
      Handler:           h,
      ReadHeaderTimeout: 5 * time.Second,
      ReadTimeout:       15 * time.Second,
      WriteTimeout:      30 * time.Second,
      IdleTimeout:       60 * time.Second,
  }
  ```
  Diese Werte sind so gewählt, dass reguläre API-Aufrufe weiterhin funktionieren.

### 2. Unbegrenzte Ressourcennutzung durch POST /pastes (Medium)
- **Betroffene Stelle:** `store.go` (`Create`, `Store` insgesamt) in Verbindung mit `handlers.go` (`handleCreatePaste`)
- **Beschreibung:** Jeder Client kann ohne Authentifizierung beliebig viele Pastes mit bis zu 1 MiB Inhalt anlegen. Der Speicher wächst unbegrenzt, bis der Arbeitsspeicher des Prozesses erschöpft ist (Speicher-DoS). Es existieren weder Ratenlimits noch ein Limit für die Anzahl der Pastes oder den Gesamtspeicher.
- **Konkreter Fix:**
  - In `Store` eine maximale Anzahl von Pastes (z. B. `maxPastes = 1000`) oder ein Gesamtspeicherlimit einführen und bei Überschreitung einen Fehler zurückgeben, den der Handler als `507 Insufficient Storage` oder `429 Too Many Requests` abbildet.
  - Optional: IP-basiertes Rate-Limiting im Handler (z. B. mit `golang.org/x/time/rate`), um Massenanlage zu unterbinden.
  - Wichtig: Das Produkt muss unter diesen Limits korrekt funktionieren – ein sinnvolles Limit (z. B. 1000 Pastes) erlaubt weiterhin den normalen Betrieb, verhindert aber die Speichererschöpfung.

### 3. DELETE ohne Autorisierung (Low)
- **Betroffene Stelle:** `handlers.go`, Funktionen `routePasteByID` und `handleDeletePaste`
- **Beschreibung:** Jeder, der die (128-Bit-zufällige) Paste-ID kennt, kann den Paste per `DELETE` entfernen. Durch die hohe Entropie ist die ID praktisch nicht erratbar, aber sobald ein Paste-Link geteilt wird, kann jeder den Paste löschen. Das ist für ein öffentliches Pastebin oft akzeptabel, stellt aber eine fehlende Autorisierung dar.
- **Konkreter Fix:**
  - Falls unerwünscht: Bei `POST` einen Eigentümer-Token (mindestens 128 Bit, ebenfalls aus `crypto/rand`) generieren, in der Antwort zurückgeben und im Store speichern. `DELETE` verlangt dann diesen Token (z. B. als `Authorization`-Header). Dadurch kann nur der Ersteller löschen. Die API bleibt funktionsfähig, da der Token dem Besitzer mitgeteilt wird.
  - Alternativ dokumentieren, dass `DELETE` bewusst ohne Auth ist und die ID als ausreichender Schutz dient.

### 4. Unverschlüsselter Transport (HTTP) (Low)
- **Betroffene Stelle:** `main.go`, `server.ListenAndServe()`
- **Beschreibung:** Die API wird ausschließlich über unverschlüsseltes HTTP ausgeliefert. Paste-Inhalte und IDs können im Netzwerk mitgelesen werden. Für den produktiven Einsatz sollte TLS verwendet werden.
- **Konkreter Fix:**
  - Bei direktem Betrieb: `server.ListenAndServeTLS("cert.pem", "key.pem")` mit gültigen Zertifikaten.
  - Oder hinter einem Reverse-Proxy (z. B. nginx/Caddy) betreiben, der HTTPS terminiert und auf den internen HTTP-Port weiterleitet. Die Anwendung selbst benötigt keine Änderung; der Proxy validiert den Transport.

## Als unkritisch/keine Findings bewertet
- `writeError` und `writeJSON` erzeugen ausschließlich JSON-Fehlerobjekte ohne interne Details (AC-11/AC-14 erfüllt).
- `crypto/rand`-basierte ID-Erzeugung mit 16 Bytes / 32 Hex-Zeichen ist korrekt (AC-12/AC-15).
- `http.MaxBytesReader` verhindert das vollständige Einlesen zu großer Bodies (AC-10 erfüllt).
- Pfadbehandlung unterbindet Traversal über `strings.Contains(id, "/")`.
- `Content-Type` wird bei allen JSON-Antworten korrekt auf `application/json` gesetzt.

**Hinweis:** Da für dieses Projekt keine Security-Scanner ausgeführt wurden (`no applicable security scanners`), beruht die Bewertung auf manueller Analyse. Das Fehlen von Scanner-Ausgaben ist kein Beleg für Abwesenheit von Schwachstellen, aber alle sichtbaren Bereiche wurden geprüft.