VERDICT: CHANGES_REQUESTED

## Prüfumfang

Geprüft wurden die sichtbaren Quelldateien `main.go`, `handlers.go`, `store.go`, `paste.go` sowie die gezeigten Testdateien. Nicht sichtbar waren die Inhalte von `README.md`, `go.mod`, `CLAUDE.md`, `AGENTS.md`. Für ein reines `go-backend` ohne Endnutzer-UI sind Impressum/Cookie-Banner/Barrierefreiheit nicht anwendbar; DSGVO und CRA sind dagegen relevant.

---

## 1. DSGVO

### 1.1 Unverschlüsselte Übertragung von Paste-Inhalten
**Schwere:** hoch  
**Datei:** `main.go`

Der Server startet ausschließlich mit `server.ListenAndServe()` und damit unverschlüsselt als HTTP. Der `content` eines Paste kann eindeutig personenbezogene Daten enthalten. Werden diese ohne TLS übertragen, sind sie auf dem Transportweg lesbar.

**Maßnahme:**
- Entweder TLS direkt im Produkt anbieten, z. B.:
  ```go
  if os.Getenv("TLS_CERT_FILE") != "" && os.Getenv("TLS_KEY_FILE") != "" {
      err = server.ListenAndServeTLS(os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE"))
  } else {
      log.Println("WARN: serving without TLS; TLS termination via reverse proxy is mandatory")
      err = server.ListenAndServe()
  }
  ```
- Oder in `README.md`/`SECURITY.md` verbindlich dokumentieren, dass der Dienst **nur** hinter einem TLS-terminierenden Reverse-Proxy betrieben werden darf. Ein direkt exponierter HTTP-Port ist als unsicherer Default nicht akzeptabel.

### 1.2 Abgelaufene Pastes bleiben im Speicher
**Schwere:** hoch  
**Datei:** `store.go`, Funktion `List()`

`List()` verwendet `RLock()` und filtert abgelaufene Pastes nur aus (`continue`). Der Datensatz wird **nicht** aus `s.pastes` gelöscht. In Kombination mit `Get()` wird ein abgelaufener Datensatz nur gelöscht, wenn genau diese ID später noch einmal angefragt wird. Abgelaufene `content`-Daten bleiben daher unbefristet im RAM, obwohl `expires_in_seconds` genau diese Speicherdauer begrenzen sollte. Das verstößt gegen den Grundsatz der Speicherbegrenzung nach Art. 5 Abs. 1 lit. e DSGVO und das ausdrückliche Ablaufmerkmal des Produkts.

**Maßnahme:**
```go
func (s *Store) List() []Metadata {
    now := time.Now().UTC()

    s.mu.Lock()
    defer s.mu.Unlock()

    metas := make([]Metadata, 0, len(s.pastes))
    for id, p := range s.pastes {
        if p.ExpiresAt != nil && !p.ExpiresAt.After(now) {
            delete(s.pastes, id)
            continue
        }
        metas = append(metas, Metadata{
            ID:        p.ID,
            Language:  p.Language,
            CreatedAt: p.CreatedAt,
            ExpiresAt: p.ExpiresAt,
        })
    }

    sort.Slice(metas, func(i, j int) bool {
        return metas[i].CreatedAt.After(metas[j].CreatedAt)
    })
    return metas
}
```
Alternativ oder zusätzlich einen periodischen Cleanup-Job vorsehen, damit abgelaufene Daten auch ohne erneuten Abruf entfernt werden.

### 1.3 Unbegrenzte In-Memory-Speicherung nicht ablaufender Pastes
**Schwere:** mittel  
**Datei:** `store.go`, `handlers.go`

`expires_in_seconds` ist optional. Ohne Wert bleibt ein Paste unbegrenzt im Speicher. Es gibt kein Gesamtkapazitätslimit, keine maximale Anzahl Pastes und keinen Löschmechanismus außer `DELETE` oder Prozessende. Für einen öffentlich erreichbaren Dienst kann das zum unkontrollierten Speicherwachstum und bei entsprechenden Inhalten zu unverhältnismäßig langer Speicherung führen.

**Maßnahme:**
- Im `Store` eine konfigurierbare Obergrenze einführen, z. B. maximale Anzahl Pastes und/oder maximale Gesamtgröße.
- Bei Erreichen der Grenze `Create` mit einem definierten Fehler abbrechen und im Handler mit `507 Insufficient Storage` oder `503 Service Unavailable` als JSON-Fehler antworten.
- In `README.md` eine dokumentierte maximale Aufbewahrung für Pastes ohne Ablauf ergänzen, z. B. Prozesslebensdauer, sofern das gewollte Betriebsmodell das ist.

### 1.4 Fehlende Datenschutzdokumentation und Rechtsgrundlage
**Schwere:** mittel  
**Datei:** `README.md` oder neu anzulegende `PRIVACY.md`

Aus den gezeigten Dateien ist keine Datenschutzdokumentation ersichtlich. Der Dienst verarbeitet mit dem Paste-`content` potenziell personenbezogene Daten. Es fehlt eine nachvollziehbare Festlegung von Rechtsgrundlage, Zwecken, Löschfristen und Betroffenenrechten.

**Maßnahme:**
- `README.md` oder `PRIVACY.md` ergänzen mit:
  - Rechtsgrundlage, z. B. Art. 6 Abs. 1 lit. b DSGVO für den bereitgestellten Dienst, sofern der Nutzer den Paste selbst anlegt.
  - Verarbeitete Datenkategorien: `content`, `language`, `created_at`, `expires_at`.
  - Speicherdauer: nur im RAM, optional ablaufend; kein Logging von IP-Adressen oder User-Agent.
  - Betroffenenrechte: Löschung über `DELETE /pastes/{id}`; Auskunft/Abruf über `GET /pastes/{id}`; keine Berichtigung von Pastes, dafür Neuanlage und Löschung.
  - Verantwortlicher und Kontakt für Datenschutzanfragen.
- Da die API kein eigenes UI hat, müssen diese Hinweise an geeigneter Stelle beim Betrieb des Dienstes bereitgestellt werden.

### 1.5 Kein `Cache-Control: no-store` für inhaltsbezogene Antworten
**Schwere:** niedrig  
**Datei:** `handlers.go`

`GET /pastes/{id}` und die `POST /pastes`-Antwort liefern den Klartext-`content` ohne `Cache-Control: no-store`. Zwischengeschaltete Caches oder Proxys könnten personenbezogene Inhalte speichern.

**Maßnahme:**
```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "no-store")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}
```
Mindestens für alle Antworten, die `content` enthalten, setzen.

### Positiv festgestellt

- Die Paste-ID wird über `crypto/rand` erzeugt und ist ein 32-stelliger Hex-String mit 128 Bit Entropie.
- Es gibt keine Log-Aufrufe, die `content`, IP-Adressen oder User-Agent ausgeben.
- Fehlerantworten enthalten nur das JSON-Feld `error`; keine Stacktraces, Dateipfade oder Teile des Inhalts.
- `GET /pastes` liefert nur Metadaten ohne `content`.
- Die Body-Größe wird mit `http.MaxBytesReader` auf 1 MiB begrenzt.

---

## 2. EU Cyber Resilience Act (CRA)

### 2.1 Fehlende TLS-Transportverschlüsselung
**Schwere:** hoch  
**Datei:** `main.go`

Wie unter DSGVO 1.1 beschrieben, fehlt ein sicherer Transportstandard. Für ein Produkt mit digitalen Elementen ist Verschlüsselung bei der Übertragung Teil der grundlegenden Sicherheitsanforderungen.

**Maßnahme:** siehe 1.1.

### 2.2 Fehlende Server-Timeouts
**Schwere:** mittel  
**Datei:** `main.go`

Der `http.Server` wird ohne `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` betrieben. Das öffnet bekannte Ressourcenerschöpfungs- und Slowloris-Angriffsvektoren.

**Maßnahme:**
```go
server := &http.Server{
    Addr:              ":" + port,
    Handler:           h,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       60 * time.Second,
    MaxHeaderBytes:    1 << 20,
}
```
Dazu `time` in `main.go` importieren.

### 2.3 Fehlende dokumentierte Sicherheitseigenschaften, Update-/Patch-Prozess und SBOM
**Schwere:** mittel  
**Datei:** `README.md`, `go.mod`, fehlende `SECURITY.md`

Aus den sichtbaren Dateien ist keine Sicherheitsdokumentation erkennbar. Für CRA-Konformität sollten Sicherheitseigenschaften, Schwachstellenmanagement und Update-/Patch-Fähigkeit nachvollziehbar sein. Die Abhängigkeitslage ist aus `go.mod` nicht sichtbar; bei ausschließlicher Standardbibliothek wäre das SBOM trivial, muss aber dokumentiert werden.

**Maßnahme:**
- `SECURITY.md` ergänzen:
  - Unterstützte Versionen und Update-/Patch-Prozess.
  - Sicherheitskontakt oder Meldeweg für Schwachstellen.
  - Beschreibung der Sicherheitseigenschaften: TLS-Vorgabe, Body-Limit, sichere ID-Erzeugung, keine Protokollierung von Inhalten, In-Memory-Modell, Expiry-Verhalten.
- `go.mod` sichtbar prüfen und alle effektiven Modulabhängigkeiten festhalten; im CI ein SBOM erzeugen, z. B. mit `cyclonedx-gomod` oder `spdx-sbom-generator`.
- Sicherstellen, dass bei zukünftigen Abhängigkeiten immer `go.sum` vorhanden und versioniert ist.

### 2.4 Unbegrenzte In-Memory-Ressourcennutzung
**Schwere:** mittel  
**Datei:** `store.go`

Ein unbegrenzter In-Memory-Store ist ohne Gesamtlimit anfällig für Erschöpfungsangriffe. Das Produkt benötigt eine dokumentierte und implementierte Ressourcenbegrenzung.

**Maßnahme:** siehe 1.3.

### 2.5 Sicherheitskonforme Fehlerantworten
**Positiv:** `writeError` liefert ausschließlich generische JSON-Meldungen. Interna wie Dateipfade oder Stacktraces werden nicht ausgegeben. Das entspricht den Anforderungen an sichere Fehlerbehandlung.

---

## 3. EU AI Act

**Kein Befund.** Das Produkt enthält keine KI-Funktion. Der AI Act ist nicht einschlägig.

---

## 4. Pflichttexte und UI

**Kein Befund.** Das Projekt ist eine reine REST-API ohne Endnutzer-UI. Impressum, Cookie-Banner, AGB im klassischen Sinne und ähnliche UI-bezogene Pflichten sind hier nicht anwendbar. Die datenschutzrechtliche Hinweispflicht bleibt aber bestehen und ist unter DSGVO 1.4 adressiert.

---

## 5. Barrierefreiheit

**Kein Befund.** Es gibt keine öffentliche Web-UI. Die European Accessibility Act/BITV/WCAG-Anforderungen greifen nicht für die reine JSON-API.

---

## 6. Gesamtfazit

Das Produkt hat solide Sicherheitsgrundlagen: sichere Zufalls-IDs, Body-Limit, JSON-Fehler ohne interne Details, kein Logging personenbezogener Daten, ordentliche Mutex-Absicherung und eine funktionale Ablauffilterung.

Offen sind jedoch behebbare Rechts- und Sicherheitslücken:

- TLS auf Transportebene fehlt oder ist nicht verbindlich dokumentiert.
- Abgelaufene Pastes werden nicht tatsächlich gelöscht, sondern nur ausgefiltert.
- Datenschutzdokumentation und Rechtsgrundlage sind nicht sichtbar.
- CRA-Dokumentation, Server-Timeouts und Ressourcenbegrenzung fehlen.

Das sind behebbare Mängel, daher: **CHANGES_REQUESTED**.