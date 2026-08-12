```
██      ██      ██      ████████      ██████    ████████        ██
██      ██    ██  ██    ██      ██  ██      ██  ██      ██    ██  ██
██      ██  ██      ██  ██      ██  ██      ██  ██      ██  ██      ██
██      ██  ██      ██  ████████    ██      ██  ████████    ██      ██
██      ██  ██████████  ██          ██      ██  ██    ██    ██████████
  ██  ██    ██      ██  ██          ██      ██  ██      ██  ██      ██
    ██      ██      ██  ██            ██████    ██      ██  ██      ██
```

[English](../../README.md) · [Español](../es/README.md) · [中文](../zh/README.md) · [日本語](../ja/README.md) · [Português](../pt/README.md) · [العربية](../ar/README.md) · [Français](../fr/README.md) · **Italiano** · [Deutsch](../de/README.md) · [Русский](../ru/README.md)

### Chatta direttamente dal tuo computer al suo. Nessun server. Nessun account. Nessuna traccia.

Condividi una riga di testo. L'altra persona la incolla. State già parlando —
cifrato, diretto, senza niente in mezzo.

[![release](https://img.shields.io/github/v/release/MalPr0/vapora?style=flat-square&color=e8a33d)](https://github.com/MalPr0/vapora/releases/latest)
![go](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square)
![dipendenze](https://img.shields.io/badge/dipendenze-zero-2ea043?style=flat-square)
![licenza](https://img.shields.io/badge/licenza-MIT-blue?style=flat-square)

---

## Provalo in 30 secondi

```bash
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz
./vapora punch
```

Stampa una riga. Mandala a qualcuno. Quella persona la incolla nel proprio
terminale.

<sup>Altre build: `darwin_amd64` · `linux_amd64` · `linux_arm64` · `windows_amd64.zip` — cambia il nome nell'URL. Usa `curl`, non il browser: un browser marca ciò che scarica come non affidabile e macOS poi si rifiuta di eseguirlo.</sup>

---

## Come si presenta

```
 █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄                    ● JADE HERON     31ms
 █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █                   ● SWIFT OTTER    47ms
 ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█                   ◐ GREY MARTEN  no reply 9s
   ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ you are CRIMSON QUAIL ━━━━━━━━━━━━━━━━━━━━━━━━━

  --             JADE HERON joined
  JADE HERON     c'è nessuno?
  SWIFT OTTER    @QUAIL guarda qua
▸ CRIMSON QUAIL  arrivo
  GREY MARTEN    ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
> hola_
                        enter sends · pgup/pgdn scrolls · !exit quits
```

Una chat da terminale in pixel art retrò. A ognuno tocca un nome di animale che
nessuno può rivendicare, le `@menzioni` tirano fuori una riga dallo scorrimento,
e un piccolo corridore attraversa la schermata di caricamento mentre la
connessione si apre la strada.

---

## Perché potrebbe servirti

**Non c'è nessuno in mezzo.** Le tue parole vanno dalla tua macchina alla sua.
Non passano per i server di un'azienda né per i miei. Non c'è un mezzo da citare
in giudizio, da vendere o da violare.

**Non c'è niente a cui iscriversi.** Niente email, niente numero, niente nome
utente, niente profilo. Il programma non sa chi sei, e nemmeno nessun altro.

**Non viene salvato nulla.** Lo chiudi e la conversazione sparisce da entrambe le
parti. Non c'è cronologia che possa trapelare, perché non esiste cronologia.

**Un file, zero dipendenze.** Scarichi un binario e lo esegui. Niente Docker,
niente runtime, niente installazione. Costruito con la libreria standard di Go e
nient'altro — puoi leggere ogni riga di ciò che viene distribuito.

**Cifrato di default, senza modo di disattivarlo.** AES-256-GCM, una chiave
diversa per ogni direzione. L'invito che condividi *è* la chiave.

**I gruppi sono una vera mesh.** Tutti parlano con tutti direttamente. Due
persone in una stanza da cinque hanno un canale che le altre tre non possono
leggere — non come promessa di comportamento, ma per aritmetica: non hanno le
chiavi.

---

## A cosa serve alla gente

- **Mandare qualcosa di delicato** a un collega senza che resti per sempre nel
  log di chat di un'azienda.
- **Parlare attraverso un firewall** dove non puoi aprire porte né installare
  niente.
- **Un canale rapido con qualcuno** che non lascia account, né cronologia, né
  tracce su nessuna delle due macchine.
- **Capire la tua connessione** — la diagnostica ti dice sulla tua rete più di
  quanto faccia il tuo operatore.

---

## In due

```bash
./vapora punch                 # tu: stampa un invito
./vapora punch "<l'invito>"    # l'altra persona: lo incolla ed esegue
```

**Se non si connette, mandatevi un invito a testa.** I router domestici di solito
rifiutano i pacchetti degli sconosciuti, quindi quando entrambi fanno così il
primo pacchetto di ciascuno muore sulla porta dell'altro. Lo schermo dell'altra
persona stampa una riga sotto *"if it does not connect, send this back"* —
fattela mandare, incollala nel tuo terminale, e ora state bussando entrambi nello
stesso momento. È esattamente ciò che quei router devono vedere.

Puoi scoprire in anticipo se ti servirà quel passaggio — vedi
[diagnostica](#conosci-la-tua-rete-prima-di-darle-la-colpa).

## In gruppo

```bash
./vapora room                  # apre una stanza e stampa un invito
./vapora room "<l'invito>"     # chiunque entra con quello
```

**Chiunque può invitare.** Sei entrato cinque minuti fa? `!invite` ti dà una riga
per portare dentro la prossima persona. Tutti finiscono per conoscere tutti senza
tornare da chi ha aperto la stanza.

**Chi ti ha invitato non è un server.** Presenta due persone e si fa da parte.
Non trasporta niente tra loro e non potrebbe leggerlo nemmeno volendo. Spegni la
macchina che ha aperto la stanza e la conversazione va avanti lo stesso.

**Le stanze contengono otto persone**, e **si chiudono quando si svuotano** — una
stanza in cui non c'è nessuno è una porta senza padrone. Aggiungi `-standalone`
se vuoi che una resti ad aspettare.

**Siete in due sullo stesso wifi?** Funziona anche così. Ogni partecipante
annuncia sia l'indirizzo pubblico sia quello locale, perché due macchine dietro
lo stesso router non riescono a raggiungersi tramite quello pubblico. Si risolve
da solo in pochi secondi.

### Mentre sei dentro

| | |
|---|---|
| `@nome` | tira la tua riga fuori dallo scorrimento dell'altro, con un segno a margine |
| `!who` | chi c'è, e quanto è in salute ogni connessione |
| `!invite` | un invito nuovo per portare dentro qualcuno |
| `!exit` | uscire, avvisando tutti subito |
| `PgUp` / `PgDn` | tornare indietro su quello che è stato detto |
| `-plain` | righe semplici invece dello schermo intero, per quando qualcosa non va |

---

## Come funziona

Il tuo computer non ha un indirizzo proprio su internet. Ce l'ha il tuo router, e
tutto ciò che è in casa lo condivide. Questo è il **NAT**, ed è il motivo per cui
nessuno può semplicemente "chiamare" il tuo portatile. La risposta abituale è
mettere un server in mezzo a cui entrambe le parti si connettono *verso l'esterno*
— funziona, e vuol dire che il computer di qualcun altro vede ogni parola.

vapora fa l'altra cosa. Entrambe le parti mandano pacchetti *verso l'esterno*
nello stesso momento, ciascuna bucando il proprio router, e i due buchi si
allineano. Dopodiché il percorso è diretto e non c'è nessun altro sopra.

| Cosa | Perché c'è |
|---|---|
| **UDP hole punching** | Il percorso diretto stesso. Entrambe le parti bucano insieme e si incontrano nel mezzo. |
| **STUN** ([5389](https://www.rfc-editor.org/rfc/rfc5389), [5780](https://www.rfc-editor.org/rfc/rfc5780)) | Scopre quale indirizzo vede il mondo esterno, e classifica il comportamento del router. |
| **UPnP-IGD, PCP, NAT-PMP** | Tre lingue per chiedere a un router di aprire una porta. Le prova tutte e tre, perché i router raramente concordano su quale parlino. |
| **X25519 + HKDF + AES-256-GCM** | Una chiave separata per coppia e per direzione. In una stanza, nessun membro legge il traffico di un'altra coppia. |
| **Finestra anti-replay** | Finestra scorrevole in stile IPsec, per mittente, così un pacchetto catturato non può esserti rigiocato contro. |
| **DHT di BitTorrent** *(facoltativo)* | Trovarsi senza alcun indirizzo. Disattivato di default — vedi [sicurezza](#sicurezza). |

Tutto dalla libreria standard di Go. Nessun codice di terze parti, da nessuna
parte.

<sup><a href="../../ARCHITECTURE.md">ARCHITECTURE.md</a> ha il passo passo, con diagrammi.</sup>

---

## Conosci la tua rete prima di darle la colpa

```bash
./vapora nat                   # che tipo di router hai davanti
./vapora diag                  # ogni router tra te e internet
```

`nat` stampa un profilo breve tipo `CONE-PORT-18`. Mandalo a chi vuoi
raggiungere, inserisci il suo, e ti dice cosa aspettarti **prima** che tu perda
una serata:

```bash
./vapora nat -pair CONE-OPEN-64                    # in due
./vapora nat -room "CONE-PORT-18,SYM-PORT-F3"      # per una stanza intera
```

Che una connessione funzioni è una proprietà della *coppia*, non di uno dei due
capi — nessuna misura della tua rete può rispondere da sola. Per questo il
profilo è fatto per essere incollato a qualcun altro. Per una stanza va oltre:
dice se la mesh si chiude, chi dovrebbe ospitarla, ed esattamente quale coppia
non si raggiungerà mai.

<sup>Se un firewall apre una porta specifica, misura quella: <code>vapora nat -port 41000</code>. Il filtraggio è una proprietà di una porta, non di una macchina.</sup>

---

## Sicurezza

**L'invito è la chiave.** Quella stringa non è un indirizzo, è il segreto che
cifra tutto. Trattalo come una password: chiunque lo veda — in uno screenshot, in
un gruppo, da sopra la spalla — può usarlo.

**Silenzio verso gli sconosciuti.** I pacchetti senza la chiave giusta non
ricevono alcuna risposta. Uno scanner di porte impara esattamente quello che
imparerebbe da una porta chiusa. Però vengono contati, e **te lo diciamo**,
perché vuol dire che qualcuno ha trovato un indirizzo che sarebbe dovuto stare
solo su un invito.

**Nessuno può prendersi la tua conversazione.** Passa il tuo invito a una terza
persona e comunque non riuscirà a scalzare il tuo amico. Il programma li
distingue, ignora il nuovo arrivato e ti avvisa.

**Passa solo testo.** Qualunque altra cosa viene scartata anziché mostrata. E al
testo che arriva dalla rete vengono tolte le sequenze di escape che
permetterebbero a qualcuno di muovere il tuo cursore, ridisegnare il tuo schermo
o arrivare ai tuoi appunti.

**Un invito resta valido finché non chiudi il programma.** Non scade e non c'è
modo di revocarlo. Chiudere e riaprire *è* la revoca — ti dà una chiave nuova e
di solito un indirizzo nuovo.

**In una stanza, un membro può mentire su chi altro è presente.** Può annunciare
qualcuno che non esiste. Quello che non può fare è leggere o falsificare ciò che
si dicono altre due persone. Un membro inventato non risponde mai e cade da solo.

**"Senza account" non è lo stesso che invisibile.** La persona con cui parli vede
il tuo indirizzo IP. Deve vederlo — i pacchetti vanno da casa tua a casa sua. È
questo che significa *diretto*, ed è lo scambio onesto per non avere un server.

**`-discover` pubblica il tuo indirizzo su una rete pubblica**, ed è per questo
che è disattivato di default. Con quella opzione, entrambe le parti si trovano
tramite il DHT di BitTorrent sotto un nome derivato dal tuo segreto. Nessuno può
cercarti senza quel segreto, ma diventi una riga in più in una tabella che
chiunque può scandagliare.

---

## Cosa si romperà, e quando

Limiti onesti, non caratteri piccoli.

- **I server STUN sono di altri** — Google, Cloudflare e altri due, servizi
  gratuiti che esistono per altro. Se spariscono, questo non riesce a scoprire il
  proprio indirizzo, e oggi non c'è un'alternativa.
- **Alcune reti lo bloccano del tutto**: aziende, università, hotel, alcuni
  operatori mobili. Dalla tua parte non c'è niente da fare.
- **Alcune connessioni proprio non ce la fanno.** Un NAT *simmetrico* o di
  operatore rende il tuo indirizzo imprevedibile da un momento all'altro, quindi
  non c'è niente a cui mirare. `vapora nat` te lo dice. L'unica soluzione è un
  relay, che questo deliberatamente non ha.
- **Il tuo indirizzo cambia e l'invito muore.** Cambi wifi, passi ai dati mobili,
  o resti fermo abbastanza a lungo. Il programma se ne accorge e ne stampa uno
  nuovo, ma sei tu a doverlo rimandare.
- **Le versioni devono coincidere.** Il formato è cambiato più volte e cambierà
  ancora. Vecchio e nuovo non si capiscono, e il sintomo è il *silenzio*. Prima
  eseguite entrambi `./vapora version`.
- **Niente è protetto a posteriori.** Chi registra il tuo traffico oggi e ottiene
  il tuo invito dopo può leggere quella registrazione. Gli strumenti seri
  risolvono la cosa con chiavi buttate via strada facendo. Questo no.
- **I binari non sono firmati.** Il tuo sistema ti avviserà, e fa bene. Verifica
  il checksum con `SHA256SUMS`, oppure compilalo tu.
- **`vapora serve` cambia la configurazione del tuo router.** È la demo originale
  di UPnP, e l'unico comando qui che chiede al router di aprire una porta verso
  internet. La richiude all'uscita — ma se va in crash, quella porta può restare
  aperta finché non riavvii il router. Tutto il resto di questo README non tocca
  il tuo router.
- **Nessuno che rompa software per mestiere ha revisionato tutto questo.** Essere
  costruito con cura non è la stessa cosa di essere sottoposto ad audit. Non
  scommetterci niente che conti.

---

## Come puoi usarlo

<sup><code>ARCHITECTURE</code> e il tutorial di Pong, collegati qui sotto, per ora esistono solo in inglese.</sup>

La chat è una cosa costruita sul canale, non il suo scopo. Il trasporto è uno
strato separato che non ha idea di cosa sia una conversazione: apre un percorso
cifrato attraverso due router, tiene viva una mesh, e sposta **byte**.

Quaranta righe sono già un programma che funziona — due copie, su due macchine in
qualunque punto di internet, che si mandano byte senza niente in mezzo:

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
mux := punch.NewMux(conn)
session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)

session.Observe(punch.ObserverFunc(func(payload []byte) {
    fmt.Println("←", string(payload))       // esattamente quello che hanno mandato
}))

go mux.Run(ctx)
go session.Run(ctx)

session.Open(ctx, 3*time.Minute)             // buca entrambi i router
session.Send([]byte("hola"))
```

### 🏓 Comincia da qui: [**costruisci un Pong**](../../examples/pong/README.md)

Un tutorial passo passo che va da quello scheletro a un vero gioco per due
attraverso internet — il suo formato di rete, chi ha il diritto di avere ragione
su cosa, e perché un gioco sopravvive a una perdita di pacchetti che rovinerebbe
una conversazione.

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

### Tre cose sullo stesso canale

| | Manda | Gli importa di |
|---|---|---|
| **[Pong](../../examples/pong/README.md)** — tutorial | **stato**, 30 volte al secondo | solo del più recente. Un pacchetto perso costa un fotogramma |
| **[filedrop](../../examples/filedrop)** | **blocchi** di un file | di tutti, e nel posto giusto |
| **`vapora punch` / `room`** | **eventi** — righe di testo | di ognuna |

Un gioco e una conversazione vogliono cose opposte dallo stesso trasporto —
freschezza contro consegna — e nessuno dei due ha avuto bisogno che il trasporto
cambiasse. È la prova più chiara che la separazione in strati è reale, ed è il
motivo per cui costruirci sopra non significa ereditare le decisioni di qualcun
altro.

### I pacchetti

| Pacchetto | Cosa ti dà |
|---|---|
| `pkg/punch` | Il percorso, la cifratura, la mesh. Byte dentro, byte fuori. |
| `pkg/stun` | Il tuo indirizzo pubblico, e una classificazione del tuo NAT. |
| `pkg/upnp`, `pkg/pcp` | Chiedere a un router di aprire una porta, in tre protocolli. |
| `pkg/dht` | Annunciare e trovare un indirizzo sul DHT di BitTorrent. |
| `pkg/diag` | Se due reti riescono a raggiungersi, e cosa fare. |
| `pkg/names` | Una chiave trasformata in un nome che una persona può pronunciare. |
| `pkg/chat` | Righe, digitazione e parlanti — lo strato che usa l'interfaccia di questo programma. |

**→ [ARCHITECTURE.md](../../ARCHITECTURE.md) percorre tutto**: come si apre un percorso
passo dopo passo, com'è fatto il formato di rete, come la mesh si genera le
chiavi, e una ricetta per costruirci sopra. Diagrammi, non prosa.

---

## Compilalo tu

La risposta più breve a "dovrei fidarmi di questo binario?":

```bash
git clone https://github.com/MalPr0/vapora && cd vapora
go build ./cmd/vapora
go test ./... -race
```

Go 1.25. Niente da scaricare, niente da configurare.

Ogni dichiarazione esportata in `pkg/` è documentata, e il controllo è nel
repository: `go run ./internal/doclint pkg`.

**L'organizzazione, se vuoi leggere.** `pkg/punch` è l'handshake, le sessioni e
le stanze. `pkg/stun` scopre il tuo indirizzo e classifica il tuo NAT.
`pkg/upnp` e `pkg/pcp` chiedono ai router di aprire porte. `pkg/dht` è il client
BitTorrent. `pkg/diag` è il ragionamento dietro i consigli. `internal/tui` è la
chat in pixel art.

[`ARCHITECTURE.md`](../../ARCHITECTURE.md) è la visita guidata a tutto questo.
[`AGENTS.md`](../../AGENTS.md) documenta le invarianti — le cose che sembrano
dettagli e reggono l'edificio. Quello è solo in inglese, perché è il riferimento
di lavoro del codice.

---

<sup>Licenza MIT. Costruito allo scoperto, un commit alla volta.</sup>
