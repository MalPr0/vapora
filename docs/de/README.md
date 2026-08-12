```
██      ██      ██      ████████      ██████    ████████        ██
██      ██    ██  ██    ██      ██  ██      ██  ██      ██    ██  ██
██      ██  ██      ██  ██      ██  ██      ██  ██      ██  ██      ██
██      ██  ██      ██  ████████    ██      ██  ████████    ██      ██
██      ██  ██████████  ██          ██      ██  ██    ██    ██████████
  ██  ██    ██      ██  ██          ██      ██  ██      ██  ██      ██
    ██      ██      ██  ██            ██████    ██      ██  ██      ██
```

[English](../../README.md) · [Español](../es/README.md) · [中文](../zh/README.md) · [日本語](../ja/README.md) · [Português](../pt/README.md) · [العربية](../ar/README.md) · [Français](../fr/README.md) · [Italiano](../it/README.md) · **Deutsch** · [Русский](../ru/README.md)

### Direkt von deinem Rechner zu ihrem. Kein Server. Kein Konto. Keine Spur.

Du teilst eine Zeile Text. Die andere Person fügt sie ein. Ihr redet bereits —
verschlüsselt, direkt, ohne irgendetwas dazwischen.

[![release](https://img.shields.io/github/v/release/MalPr0/vapora?style=flat-square&color=e8a33d)](https://github.com/MalPr0/vapora/releases/latest)
![go](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square)
![Abhängigkeiten](https://img.shields.io/badge/Abhängigkeiten-null-2ea043?style=flat-square)
![Lizenz](https://img.shields.io/badge/Lizenz-MIT-blue?style=flat-square)

---

## In 30 Sekunden ausprobieren

```bash
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz
./vapora punch
```

Es gibt eine Zeile aus. Schick sie jemandem. Diese Person fügt sie in ihr
Terminal ein.

<sup>Andere Builds: `darwin_amd64` · `linux_amd64` · `linux_arm64` · `windows_amd64.zip` — einfach den Namen in der URL austauschen. Nimm `curl`, nicht den Browser: ein Browser markiert Heruntergeladenes als nicht vertrauenswürdig, und macOS weigert sich danach, es auszuführen.</sup>

---

## Wie es aussieht

```
 █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄                    ● JADE HERON     31ms
 █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █                   ● SWIFT OTTER    47ms
 ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█                   ◐ GREY MARTEN  no reply 9s
   ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ you are CRIMSON QUAIL ━━━━━━━━━━━━━━━━━━━━━━━━━

  --             JADE HERON joined
  JADE HERON     ist da jemand?
  SWIFT OTTER    @QUAIL schau dir das an
▸ CRIMSON QUAIL  bin dabei
  GREY MARTEN    ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
> hola_
                        enter sends · pgup/pgdn scrolls · !exit quits
```

Ein Terminal-Chat in Retro-Pixel-Art. Jede Person bekommt einen Tiernamen, den
niemand für sich beanspruchen kann, `@Erwähnungen` heben eine Zeile aus dem
Verlauf hervor, und ein kleiner Läufer sprintet über den Ladebildschirm, während
sich die Verbindung durchschlägt.

---

## Warum das für dich interessant sein könnte

**Niemand ist dazwischen.** Deine Worte gehen von deinem Rechner zu ihrem. Nicht
über die Server irgendeiner Firma, nicht über meine. Es gibt keine Mitte, die man
vorladen, verkaufen oder aufbrechen kann.

**Es gibt nichts zu registrieren.** Keine E-Mail, keine Telefonnummer, kein
Benutzername, kein Profil. Das Programm weiß nicht, wer du bist, und sonst weiß
es auch niemand.

**Nichts wird gespeichert.** Schließen, und das Gespräch ist auf beiden Seiten
weg. Es gibt keinen Verlauf, der auslaufen könnte, weil es keinen Verlauf gibt.

**Eine Datei, null Abhängigkeiten.** Binary herunterladen und starten. Kein
Docker, keine Laufzeitumgebung, keine Installation. Gebaut allein aus der
Go-Standardbibliothek — du kannst jede ausgelieferte Zeile lesen.

**Standardmäßig verschlüsselt, ohne Möglichkeit, es abzuschalten.** AES-256-GCM,
für jede Richtung ein eigener Schlüssel. Die Einladung, die du teilst, *ist* der
Schlüssel.

**Gruppen sind ein echtes Mesh.** Alle sprechen direkt miteinander. Zwei Leute in
einem Raum von fünf haben einen Kanal, den die anderen drei nicht lesen können —
nicht als Versprechen über Verhalten, sondern als Arithmetik: sie haben die
Schlüssel nicht.

---

## Wofür Leute es benutzen

- **Etwas Heikles an eine Kollegin schicken**, ohne dass es für immer im
  Chat-Log einer Firma liegt.
- **Durch eine Firewall hindurch reden**, dort wo du weder Ports öffnen noch
  irgendetwas installieren kannst.
- **Ein schneller Kanal mit jemandem**, der kein Konto, keinen Verlauf und auf
  keinem der beiden Rechner eine Spur hinterlässt.
- **Die eigene Verbindung verstehen** — die Diagnose sagt dir mehr über dein
  Netz als dein Anbieter.

---

## Zu zweit

```bash
./vapora punch                       # du: gibt eine Einladung aus
./vapora punch "<die Einladung>"     # die andere Person: einfügen und ausführen
```

**Wenn es nicht verbindet, schickt euch beide eine Einladung.** Heimrouter lehnen
Pakete von Fremden meist ab, also stirbt, wenn beide das tun, das jeweils erste
Paket an der Tür des anderen. Auf dem Bildschirm der anderen Person erscheint
unter *"if it does not connect, send this back"* eine Zeile — lass sie dir
schicken, füg sie in dein Terminal ein, und jetzt klopft ihr beide im selben
Moment. Genau das müssen diese Router sehen.

Du kannst vorher herausfinden, ob du diesen Schritt brauchst — siehe
[Diagnose](#kenn-dein-netz-bevor-du-ihm-die-schuld-gibst).

## In der Gruppe

```bash
./vapora room                        # öffnet einen Raum und gibt eine Einladung aus
./vapora room "<die Einladung>"      # alle treten damit bei
```

**Jede Person kann einladen.** Vor fünf Minuten dazugekommen? `!invite` gibt dir
eine Zeile, um die nächste Person zu holen. Am Ende kennen alle alle, ohne dass
jemand zu demjenigen zurück muss, der den Raum geöffnet hat.

**Wer dich eingeladen hat, ist kein Server.** Er stellt zwei Leute einander vor
und tritt zur Seite. Er trägt nichts zwischen ihnen hin und her und könnte es
auch nicht lesen, wenn er wollte. Schalte den Rechner aus, der den Raum geöffnet
hat, und das Gespräch läuft ohne ihn weiter.

**Räume fassen acht**, und sie **schließen, sobald sie leer sind** — ein Raum, in
dem niemand ist, ist ein Port ohne Besitzer. Mit `-standalone` bleibt einer
wartend offen.

**Ihr seid zu zweit im selben WLAN?** Das geht auch. Jede Teilnehmerin
veröffentlicht sowohl ihre öffentliche als auch ihre lokale Adresse, weil zwei
Rechner hinter demselben Router einander über die öffentliche nicht erreichen.
Das regelt sich in wenigen Sekunden von selbst.

### Während du drin bist

| | |
|---|---|
| `@name` | hebt deine Zeile aus dem Verlauf der anderen Person hervor, mit einer Markierung am Rand |
| `!who` | wer da ist, und wie gesund jede Verbindung ist |
| `!invite` | eine frische Einladung, um jemanden zu holen |
| `!exit` | gehen, und es allen sofort sagen |
| `PgUp` / `PgDn` | im Gesagten zurückblättern |
| `-plain` | einfache Zeilen statt Vollbild, für den Fall, dass etwas nicht stimmt |

---

## Wie es funktioniert

Dein Rechner hat im Internet keine eigene Adresse. Die hat dein Router, und alles
bei dir zu Hause teilt sie sich. Das ist **NAT**, und deshalb kann niemand deinen
Laptop einfach „anrufen". Die übliche Antwort ist ein Server in der Mitte, zu dem
beide Seiten *hinaus* verbinden — das funktioniert, und es bedeutet, dass der
Rechner von jemand anderem jedes Wort sieht.

vapora macht das andere. Beide Seiten schicken im selben Moment Pakete *hinaus*,
jede schlägt ein Loch durch ihren eigenen Router, und die beiden Löcher liegen
übereinander. Danach ist der Weg direkt und niemand sonst ist darauf.

| Was | Warum es da ist |
|---|---|
| **UDP Hole Punching** | Der direkte Weg selbst. Beide Seiten schlagen gleichzeitig durch und treffen sich in der Mitte. |
| **STUN** ([5389](https://www.rfc-editor.org/rfc/rfc5389), [5780](https://www.rfc-editor.org/rfc/rfc5780)) | Findet heraus, welche Adresse die Außenwelt sieht, und klassifiziert das Verhalten deines Routers. |
| **UPnP-IGD, PCP, NAT-PMP** | Drei Sprachen, um einen Router zu bitten, eine Tür zu öffnen. Alle drei werden versucht, weil Router sich selten einig sind, welche sie sprechen. |
| **X25519 + HKDF + AES-256-GCM** | Ein eigener Schlüssel pro Paar und pro Richtung. In einem Raum liest kein Mitglied den Verkehr eines anderen Paares. |
| **Anti-Replay-Fenster** | Gleitendes Fenster nach IPsec-Art, pro Absender, damit ein mitgeschnittenes Paket nicht gegen dich abgespielt werden kann. |
| **BitTorrent-DHT** *(optional)* | Sich ganz ohne Adresse finden. Standardmäßig aus — siehe [Sicherheit](#sicherheit). |

Alles aus der Go-Standardbibliothek. Nirgendwo Fremdcode.

<sup><a href="../../ARCHITECTURE.md">ARCHITECTURE.md</a> hat die Schritt-für-Schritt-Erklärung, mit Diagrammen.</sup>

---

## Kenn dein Netz, bevor du ihm die Schuld gibst

```bash
./vapora nat                   # was für ein Router vor dir steht
./vapora diag                  # jeder Router zwischen dir und dem Internet
```

`nat` gibt ein kurzes Profil aus, etwa `CONE-PORT-18`. Schick es der Person, mit
der du dich verbinden willst, trag ihres ein, und es sagt dir, was zu erwarten
ist — **bevor** du einen Abend verlierst:

```bash
./vapora nat -pair CONE-OPEN-64                    # zu zweit
./vapora nat -room "CONE-PORT-18,SYM-PORT-F3"      # für einen ganzen Raum
```

Ob eine Verbindung klappt, ist eine Eigenschaft des *Paares*, nicht eines der
beiden Enden — keine Messung deines eigenen Netzes beantwortet das allein.
Deshalb ist das Profil dafür gemacht, jemand anderem geschickt zu werden. Für
einen Raum geht es weiter: es sagt, ob das Mesh sich schließt, wer hosten sollte,
und genau welches Paar einander nie erreichen wird.

<sup>Wenn eine Firewall genau einen Port öffnet, miss diesen: <code>vapora nat -port 41000</code>. Filterung ist eine Eigenschaft eines Ports, nicht eines Rechners.</sup>

---

## Sicherheit

**Die Einladung ist der Schlüssel.** Diese Zeichenkette ist keine Adresse,
sondern das Geheimnis, das alles verschlüsselt. Behandle sie wie ein Passwort:
wer sie sieht — auf einem Screenshot, in einem Gruppenchat, über die Schulter —
kann sie benutzen.

**Schweigen gegenüber Fremden.** Pakete ohne den richtigen Schlüssel bekommen
überhaupt keine Antwort. Ein Portscanner erfährt genau das, was er von einem
geschlossenen Port erfahren würde. Sie werden aber gezählt, und **du wirst
informiert**, denn es bedeutet, dass jemand eine Adresse gefunden hat, die nur
auf einer einzigen Einladung stehen sollte.

**Niemand kann dein Gespräch übernehmen.** Gib deine Einladung an eine dritte
Person weiter, und sie kann deine Freundin trotzdem nicht verdrängen. Das
Programm unterscheidet sie, ignoriert die neue und warnt dich.

**Nur Text kommt durch.** Alles andere wird verworfen statt angezeigt. Und aus
Text aus dem Netz werden die Escape-Sequenzen entfernt, mit denen jemand deinen
Cursor bewegen, deinen Bildschirm neu zeichnen oder an deine Zwischenablage
kommen könnte.

**Eine Einladung bleibt gültig, bis du das Programm schließt.** Sie läuft nicht
ab und lässt sich nicht widerrufen. Schließen und neu öffnen *ist* der Widerruf —
das gibt dir einen neuen Schlüssel und meist eine neue Adresse.

**In einem Raum kann ein Mitglied darüber lügen, wer sonst da ist.** Es kann
jemanden ankündigen, den es nicht gibt. Was es nicht kann, ist lesen oder
fälschen, was zwei andere einander sagen. Ein erfundenes Mitglied antwortet nie
und fällt von selbst heraus.

**„Kein Konto" ist nicht dasselbe wie unsichtbar.** Die Person, mit der du
sprichst, sieht deine IP-Adresse. Sie muss es — die Pakete gehen von deinem Haus
zu ihrem. Das ist die Bedeutung von *direkt*, und der ehrliche Preis dafür,
keinen Server zu haben.

**`-discover` veröffentlicht deine Adresse in einem öffentlichen Netz**, und
deshalb ist es standardmäßig aus. Damit finden sich beide Seiten über das
BitTorrent-DHT unter einem aus deinem Geheimnis abgeleiteten Namen. Ohne dieses
Geheimnis kann dich niemand nachschlagen, aber du wirst zu einer weiteren Zeile
in einer Tabelle, die jeder durchsuchen kann.

---

## Was kaputtgehen wird, und wann

Ehrliche Grenzen, kein Kleingedrucktes.

- **Die STUN-Server gehören anderen** — Google, Cloudflare und zwei weitere,
  kostenlose Dienste, die es für etwas anderes gibt. Verschwinden sie, kann das
  hier seine eigene Adresse nicht mehr herausfinden, und heute gibt es keinen
  Ersatz.
- **Manche Netze blockieren es rundweg**: Firmen, Universitäten, Hotels, manche
  Mobilfunkanbieter. Auf deiner Seite hilft nichts dagegen.
- **Manche Anschlüsse können es schlicht nicht.** Ein *symmetrisches* oder
  Carrier-Grade-NAT macht deine Adresse von Moment zu Moment unvorhersehbar, es
  gibt also nichts, worauf man zielen könnte. `vapora nat` sagt es dir. Die
  einzige Lösung ist ein Relay, das dieses Programm bewusst nicht hat.
- **Deine Adresse ändert sich und die Einladung stirbt.** WLAN wechseln, auf
  Mobilfunk gehen, lange genug untätig bleiben. Das Programm merkt es und gibt
  eine neue aus, aber verschicken musst du sie selbst.
- **Die Versionen müssen zusammenpassen.** Das Format hat sich mehrfach geändert
  und wird sich wieder ändern. Alt und neu verstehen sich nicht, und das Symptom
  ist *Stille*. Führt beide zuerst `./vapora version` aus.
- **Nichts ist rückwirkend geschützt.** Wer heute deinen Verkehr aufzeichnet und
  später deine Einladung bekommt, kann die Aufzeichnung lesen. Ernsthafte
  Werkzeuge lösen das mit Schlüsseln, die unterwegs weggeworfen werden. Dieses
  nicht.
- **Die Binaries sind nicht signiert.** Dein Betriebssystem wird warnen, und zu
  Recht. Prüfe die Prüfsumme gegen `SHA256SUMS`, oder baue es selbst.
- **`vapora serve` ändert die Konfiguration deines Routers.** Es ist die
  ursprüngliche UPnP-Demo und der einzige Befehl hier, der den Router bittet,
  einen Port zum Internet zu öffnen. Beim Beenden schließt er ihn wieder — stürzt
  er aber ab, kann diese Tür bis zum Neustart des Routers offen bleiben. Alles
  andere in dieser README fasst deinen Router nicht an.
- **Niemand, der beruflich Software bricht, hat das hier geprüft.** Sorgfältig
  gebaut zu sein ist nicht dasselbe wie auditiert zu sein. Setz nichts Wichtiges
  darauf.

---

## Wie du es benutzen kannst

<sup><code>ARCHITECTURE</code> und das Pong-Tutorial, unten verlinkt, gibt es vorerst nur auf Englisch.</sup>

Der Chat ist eine Sache, die auf dem Kanal aufbaut, nicht sein Zweck. Der
Transport ist eine eigene Schicht, die keine Ahnung hat, was ein Gespräch ist: er
öffnet einen verschlüsselten Weg durch zwei Router, hält ein Mesh am Leben und
bewegt **Bytes**.

Vierzig Zeilen sind bereits ein laufendes Programm — zwei Kopien davon, auf zwei
Rechnern irgendwo im Internet, die einander Bytes schicken, ohne irgendetwas
dazwischen:

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
mux := punch.NewMux(conn)
session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)

session.Observe(punch.ObserverFunc(func(payload []byte) {
    fmt.Println("←", string(payload))       // genau das, was gesendet wurde
}))

go mux.Run(ctx)
go session.Run(ctx)

session.Open(ctx, 3*time.Minute)             // schlägt durch beide Router
session.Send([]byte("hola"))
```

### 🏓 Fang hier an: [**bau ein Pong**](../../examples/pong/README.md)

Ein Schritt-für-Schritt-Tutorial, das von diesem Gerüst zu einem echten Spiel für
zwei über das Internet führt — mit eigenem Netzformat, mit der Frage, wer worüber
recht haben darf, und warum ein Spiel Paketverluste übersteht, die ein Gespräch
ruinieren würden.

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

### Drei Dinge auf einem Kanal

| | Sendet | Kümmert sich um |
|---|---|---|
| **[Pong](../../examples/pong/README.md)** — Tutorial | **Zustand**, 30× pro Sekunde | nur das Neueste. Ein verlorenes Paket kostet ein Bild |
| **[filedrop](../../examples/filedrop)** | **Blöcke** einer Datei | alle, und an der richtigen Stelle |
| **`vapora punch` / `room`** | **Ereignisse** — Textzeilen | jede einzelne |

Ein Spiel und ein Gespräch wollen vom selben Transport das Gegenteil —
Aktualität gegen Zustellung — und keines von beiden brauchte eine Änderung am
Transport. Das ist der klarste Beleg dafür, dass die Schichtung echt ist, und der
Grund, warum darauf zu bauen nicht heißt, die Entscheidungen anderer zu erben.

### Die Pakete

| Paket | Was es dir gibt |
|---|---|
| `pkg/punch` | Den Weg, die Verschlüsselung, das Mesh. Bytes rein, Bytes raus. |
| `pkg/stun` | Deine öffentliche Adresse und eine Einordnung deines NAT. |
| `pkg/upnp`, `pkg/pcp` | Einen Router in drei Protokollen um eine offene Tür bitten. |
| `pkg/dht` | Eine Adresse im BitTorrent-DHT ankündigen und finden. |
| `pkg/diag` | Ob zwei Netze einander erreichen, und was zu tun ist. |
| `pkg/names` | Ein Schlüssel, verwandelt in einen Namen, den ein Mensch aussprechen kann. |
| `pkg/chat` | Zeilen, Tippen und Sprecher — die Schicht, die die Oberfläche dieses Programms benutzt. |

**→ [ARCHITECTURE.md](../../ARCHITECTURE.md) geht das Ganze durch**: wie ein Weg Schritt
für Schritt geöffnet wird, wie das Netzformat aussieht, wie das Mesh sich selbst
Schlüssel erzeugt, und ein Rezept, darauf zu bauen. Diagramme, keine Prosa.

---

## Selbst bauen

Die kürzeste Antwort auf „soll ich diesem Binary trauen?":

```bash
git clone https://github.com/MalPr0/vapora && cd vapora
go build ./cmd/vapora
go test ./... -race
```

Go 1.25. Nichts herunterzuladen, nichts zu konfigurieren.

Jede exportierte Deklaration in `pkg/` ist dokumentiert, und die Prüfung liegt im
Repository: `go run ./internal/doclint pkg`.

**Der Aufbau, falls du lesen willst.** `pkg/punch` ist der Handshake, die
Sitzungen und die Räume. `pkg/stun` findet deine Adresse heraus und ordnet dein
NAT ein. `pkg/upnp` und `pkg/pcp` bitten Router um offene Türen. `pkg/dht` ist
der BitTorrent-Client. `pkg/diag` ist die Überlegung hinter den Empfehlungen.
`internal/tui` ist der Pixel-Art-Chat.

[`ARCHITECTURE.md`](../../ARCHITECTURE.md) ist die Führung durch das Ganze.
[`AGENTS.md`](../../AGENTS.md) hält die Invarianten fest — die Dinge, die wie
Details aussehen und sich als tragend herausstellen. Das gibt es nur auf
Englisch, weil es die Arbeitsreferenz für den Code ist.

---

<sup>MIT-Lizenz. Offen gebaut, ein Commit nach dem anderen.</sup>
