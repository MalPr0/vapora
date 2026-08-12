```
██      ██      ██      ████████      ██████    ████████        ██
██      ██    ██  ██    ██      ██  ██      ██  ██      ██    ██  ██
██      ██  ██      ██  ██      ██  ██      ██  ██      ██  ██      ██
██      ██  ██      ██  ████████    ██      ██  ████████    ██      ██
██      ██  ██████████  ██          ██      ██  ██    ██    ██████████
  ██  ██    ██      ██  ██          ██      ██  ██      ██  ██      ██
    ██      ██      ██  ██            ██████    ██      ██  ██      ██
```

[English](README.md) · **Español**

### Hablá directo de tu computadora a la suya. Sin servidor. Sin cuenta. Sin rastro.

Compartís una línea de texto. La otra persona la pega. Ya están hablando —
cifrado, directo, sin nada en el medio.

[![release](https://img.shields.io/github/v/release/MalPr0/vapora?style=flat-square&color=e8a33d)](https://github.com/MalPr0/vapora/releases/latest)
![go](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square)
![dependencias](https://img.shields.io/badge/dependencias-cero-2ea043?style=flat-square)
![licencia](https://img.shields.io/badge/licencia-MIT-blue?style=flat-square)

---

## Probalo en 30 segundos

```bash
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz
./vapora punch
```

Imprime una línea. Mandásela a alguien. Esa persona la pega en su terminal.

<sup>Otras versiones: `darwin_amd64` · `linux_amd64` · `linux_arm64` · `windows_amd64.zip` — cambiá el nombre en la URL. Usá `curl`, no el navegador: un navegador marca lo que baja como no confiable y macOS después se niega a ejecutarlo.</sup>

---

## Cómo se ve

```
 █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄                    ● JADE HERON     31ms
 █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █                   ● SWIFT OTTER    47ms
 ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█                   ◐ GREY MARTEN  no reply 9s
   ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ you are CRIMSON QUAIL ━━━━━━━━━━━━━━━━━━━━━━━━━

  --             JADE HERON joined
  JADE HERON     alguien ahí?
  SWIFT OTTER    @QUAIL mirá esto
▸ CRIMSON QUAIL  voy
  GREY MARTEN    ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
> hola_
                        enter sends · pgup/pgdn scrolls · !exit quits
```

Un chat de terminal en pixel art retro. A cada persona le toca un nombre de
animal que nadie puede reclamar, las `@menciones` sacan una línea del scroll, y
un corredorcito cruza la pantalla de carga mientras la conexión se abre paso.

---

## Por qué te puede servir

**Nadie está en el medio.** Tus palabras van de tu máquina a la de la otra
persona. No pasan por los servidores de una empresa ni por los míos. No hay un
medio al que citar judicialmente, que vender, ni al que le puedan entrar.

**No hay que registrarse.** Sin mail, sin teléfono, sin usuario, sin perfil. El
programa no sabe quién sos, y nadie más tampoco.

**No se guarda nada.** Lo cerrás y la conversación desaparece de los dos lados.
No hay historial que se pueda filtrar, porque no hay historial.

**Un archivo, cero dependencias.** Bajás un binario y lo ejecutás. Sin Docker,
sin runtime, sin instalar nada. Está construido con la biblioteca estándar de Go
y nada más — podés leer cada línea de lo que se distribuye.

**Cifrado por defecto, sin forma de apagarlo.** AES-256-GCM, con una clave
distinta para cada dirección. La invitación que compartís *es* la clave.

**Los grupos son una malla de verdad.** Todos hablan con todos directamente. Dos
personas en una sala de cinco tienen un canal que las otras tres no pueden leer
— y no como promesa de comportamiento, sino por aritmética: no tienen las
claves.

---

## Para qué lo usa la gente

- **Mandar algo delicado** a un colega sin que quede para siempre en el log de
  chat de una empresa.
- **Hablar a través de un firewall** donde no podés abrir puertos ni instalar
  nada.
- **Un canal rápido con alguien** que no deja cuenta, ni historial, ni rastro en
  ninguna de las dos máquinas.
- **Entender tu propia conexión** — el diagnóstico te dice más sobre tu red que
  tu proveedor.

---

## Dos personas

```bash
./vapora punch                     # vos: imprime una invitación
./vapora punch "<la invitación>"   # la otra persona: la pega y la ejecuta
```

**Si no conecta, manden una invitación cada uno.** Los routers hogareños suelen
rechazar paquetes de desconocidos, así que cuando los dos hacen eso, el primer
paquete de cada uno muere en la puerta del otro. La pantalla de la otra persona
imprime una línea bajo *"if it does not connect, send this back"* — que te la
mande, la pegás en tu terminal, y ahora los dos están golpeando al mismo tiempo.
Eso es exactamente lo que esos routers necesitan ver.

Podés averiguar de antemano si vas a necesitar ese paso — ver
[diagnóstico](#conocé-tu-red-antes-de-echarle-la-culpa).

## Un grupo

```bash
./vapora room                      # abre una sala e imprime una invitación
./vapora room "<la invitación>"    # cualquiera entra con ella
```

**Cualquiera puede invitar.** ¿Entraste hace cinco minutos? `!invite` te da una
línea para sumar a la próxima persona. Todos terminan conociendo a todos sin
tener que volver a quien abrió la sala.

**Quien te invitó no es un servidor.** Presenta a dos personas y se corre. No
lleva nada entre ellas y no podría leerlo aunque quisiera. Apagá la máquina que
abrió la sala y la conversación sigue sin ella.

**Las salas son de ocho**, y **se cierran cuando quedan vacías** — una sala en la
que no hay nadie es un puerto sin dueño. Agregá `-standalone` si querés que una
se quede esperando.

**¿Están los dos en el mismo wifi?** También funciona. Cada participante anuncia
su dirección pública y la local, porque dos máquinas detrás del mismo router no
pueden alcanzarse por la pública. Se resuelve solo en unos segundos.

### Mientras estás adentro

| | |
|---|---|
| `@nombre` | saca tu línea del scroll de esa persona, con una marca en el margen |
| `!who` | quién está, y qué tan sana está cada conexión |
| `!invite` | una invitación nueva para sumar a alguien |
| `!exit` | salir, y avisarle a todos en el momento |
| `PgUp` / `PgDn` | volver atrás en lo que se dijo |
| `-plain` | líneas planas en vez de pantalla completa, para cuando algo anda mal |

---

## Cómo funciona

Tu computadora no tiene dirección propia en internet. La tiene tu router, y todo
lo de tu casa la comparte. Eso es **NAT**, y es la razón por la que nadie puede
simplemente llamar a tu laptop. La respuesta habitual es un servidor en el medio
al que los dos lados se conectan *hacia afuera* — funciona, y significa que la
computadora de otro ve cada palabra.

vapora hace la otra cosa. Los dos lados mandan paquetes *hacia afuera* en el
mismo momento, cada uno perforando un agujero en su propio router, y los dos
agujeros se alinean. Después de eso el camino es directo y no hay nadie más en
él.

| Qué | Por qué está |
|---|---|
| **UDP hole punching** | El camino directo en sí. Los dos lados perforan a la vez y se encuentran en el medio. |
| **STUN** ([5389](https://www.rfc-editor.org/rfc/rfc5389), [5780](https://www.rfc-editor.org/rfc/rfc5780)) | Averigua qué dirección ve el mundo exterior, y clasifica cómo se comporta tu router. |
| **UPnP-IGD, PCP, NAT-PMP** | Tres idiomas para pedirle a un router que abra una puerta. Prueba los tres, porque los routers rara vez coinciden en cuál hablan. |
| **X25519 + HKDF + AES-256-GCM** | Una clave separada por par y por dirección. En una sala, ningún miembro puede leer el tráfico de otro par. |
| **Ventana anti-replay** | Ventana deslizante estilo IPsec, por emisor, para que un paquete capturado no se pueda reproducir contra vos. |
| **DHT de BitTorrent** *(opcional)* | Encontrarse sin ninguna dirección. Apagado por defecto — ver [seguridad](#seguridad). |

Todo con la biblioteca estándar de Go. Nada de código de terceros, en ningún
lado.

<sup><a href="ARCHITECTURE.es.md">ARCHITECTURE.es.md</a> tiene el paso a paso, con diagramas.</sup>

---

## Conocé tu red antes de echarle la culpa

```bash
./vapora nat                   # qué tipo de router tenés adelante
./vapora diag                  # cada router entre vos e internet
```

`nat` imprime un perfil corto tipo `CONE-PORT-18`. Mandáselo a la persona con la
que te querés conectar, poné el de ella, y te dice qué esperar **antes** de que
pierdas una tarde:

```bash
./vapora nat -pair CONE-OPEN-64                    # para dos personas
./vapora nat -room "CONE-PORT-18,SYM-PORT-F3"      # para una sala entera
```

Que una conexión funcione es una propiedad del *par*, no de ninguno de los dos
lados — ninguna medición de tu propia red puede responderlo sola. Por eso el
perfil está hecho para pegárselo a otra persona. Para una sala va más lejos: te
dice si la malla cierra, quién debería hospedarla, y exactamente qué par nunca
se va a alcanzar.

<sup>Si un firewall abre un puerto específico, medí ese: <code>vapora nat -port 41000</code>. El filtrado es una propiedad de un puerto, no de una máquina.</sup>

---

## Seguridad

**La invitación es la clave.** Esa cadena no es una dirección, es el secreto que
cifra todo. Tratala como una contraseña: cualquiera que la vea — en una captura,
en un chat grupal, por encima del hombro — puede usarla.

**Silencio a los desconocidos.** Los paquetes sin la clave correcta no reciben
respuesta alguna. Un escáner de puertos aprende exactamente lo mismo que
aprendería de un puerto cerrado. Pero se cuentan, y **te avisamos**, porque
significa que alguien encontró una dirección que solo debería haber estado en una
invitación.

**Nadie puede tomarte la conversación.** Si le pasás tu invitación a una tercera
persona, igual no puede desplazar a tu amigo. El programa los distingue, ignora
al recién llegado y te avisa.

**Solo cruza texto.** Cualquier otra cosa se descarta en vez de mostrarse. Y al
texto que viene de la red se le quitan las secuencias de escape que le
permitirían a alguien mover tu cursor, repintarte la pantalla o llegar a tu
portapapeles.

**Una invitación sigue siendo válida hasta que cerrás el programa.** No vence y
no hay forma de revocarla. Cerrar y volver a abrir *es* la revocación — te da una
clave nueva y normalmente una dirección nueva.

**En una sala, un miembro puede mentir sobre quién más está.** Puede anunciar a
alguien que no existe. Lo que no puede hacer es leer ni falsificar lo que se
dicen otras dos personas. Un miembro inventado nunca responde y se cae solo.

**"Sin cuenta" no es lo mismo que invisible.** La persona con la que hablás ve tu
dirección IP. Tiene que verla — los paquetes van de tu casa a la suya. Eso es lo
que significa *directo*, y es el intercambio honesto por no tener servidor.

**`-discover` publica tu dirección en una red pública**, y por eso está apagado
por defecto. Con esa opción, los dos lados se encuentran a través del DHT de
BitTorrent bajo un nombre derivado de tu secreto. Nadie puede buscarte sin ese
secreto, pero pasás a ser una dirección más en una tabla que cualquiera puede
recorrer.

---

## Qué va a romperse, y cuándo

Limitaciones honestas, no letra chica.

- **Los servidores STUN son de otros** — Google, Cloudflare y dos más, servicios
  gratuitos que existen para otra cosa. Si desaparecen, esto no puede averiguar
  su propia dirección, y hoy no hay alternativa.
- **Algunas redes lo bloquean directamente**: empresas, universidades, hoteles,
  algunas operadoras móviles. Nada de tu lado lo arregla.
- **Algunas conexiones no pueden hacerlo en absoluto.** Un NAT *simétrico* o de
  operadora hace que tu dirección sea impredecible de un momento a otro, así que
  no hay a qué apuntar. `vapora nat` te lo dice. La única solución es un relay,
  que esto deliberadamente no tiene.
- **Tu dirección cambia y la invitación muere.** Cambiás de wifi, pasás a datos
  móviles, o lo dejás quieto el tiempo suficiente. El programa lo nota e imprime
  una nueva, pero la tenés que volver a mandar.
- **Las versiones tienen que coincidir.** El formato cambió varias veces y va a
  volver a cambiar. Una versión vieja y una nueva no se entienden, y el síntoma
  es *silencio*. Corran `./vapora version` los dos primero.
- **Nada está protegido hacia atrás.** Alguien que grabe tu tráfico hoy y consiga
  tu invitación después puede leer esa grabación. Las herramientas serias
  resuelven esto con claves que se descartan sobre la marcha. Esta no.
- **Los binarios no están firmados.** Tu sistema operativo te va a advertir, y
  hace bien. Verificá el checksum contra `SHA256SUMS`, o compilalo vos.
- **`vapora serve` cambia la configuración de tu router.** Es el demo original de
  UPnP, y el único comando acá que le pide al router abrir un puerto a internet.
  Lo cierra al salir — pero si se cae, esa puerta puede quedar abierta hasta que
  reinicies el router. Todo lo demás en este README no toca tu router.
- **Nadie que rompa software para vivir revisó esto.** Estar construido con
  cuidado no es lo mismo que estar auditado. No te juegues nada importante.

---

## Cómo podés usar esto

El chat es una cosa construida sobre el canal, no el objetivo del canal. El
transporte es una capa separada que no tiene idea de qué es una conversación:
abre un camino cifrado a través de dos routers, mantiene viva una malla, y mueve
**bytes**.

Cuarenta líneas ya son un programa que funciona — dos copias, en dos máquinas en
cualquier parte de internet, mandándose bytes sin nada en el medio:

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
mux := punch.NewMux(conn)
session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)

session.Observe(punch.ObserverFunc(func(payload []byte) {
    fmt.Println("←", string(payload))       // exactamente lo que mandaron
}))

go mux.Run(ctx)
go session.Run(ctx)

session.Open(ctx, 3*time.Minute)             // perfora los dos routers
session.Send([]byte("hola"))
```

### 🏓 Empezá por acá: [**construí un Pong**](examples/pong/README.es.md)

Un tutorial paso a paso que va de ese esqueleto a un juego real de dos jugadores
a través de internet — su propio formato en la red, quién tiene derecho a tener
razón sobre qué, y por qué un juego sobrevive a una pérdida de paquetes que
arruinaría una conversación.

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

### Tres cosas sobre un mismo canal

| | Manda | Le importa |
|---|---|---|
| **[Pong](examples/pong/README.es.md)** — tutorial | **estado**, 30 veces por segundo | solo el más nuevo. Un paquete perdido cuesta un cuadro |
| **[filedrop](examples/filedrop)** | **bloques** de un archivo | todos, y en el lugar correcto |
| **`vapora punch` / `room`** | **eventos** — líneas de texto | cada una de ellas |

Un juego y una conversación quieren cosas opuestas del mismo transporte —
frescura contra entrega — y ninguno necesitó que el transporte cambiara. Esa es
la evidencia más clara de que la separación en capas es real, y es la razón por
la que construir encima no significa heredar las decisiones de otro.

### Los paquetes

| Paquete | Qué te da |
|---|---|
| `pkg/punch` | El camino, el cifrado, la malla. Bytes adentro, bytes afuera. |
| `pkg/stun` | Tu dirección pública, y una clasificación de tu NAT. |
| `pkg/upnp`, `pkg/pcp` | Pedirle a un router que abra una puerta, en tres protocolos. |
| `pkg/dht` | Anunciar y encontrar una dirección en el DHT de BitTorrent. |
| `pkg/diag` | Si dos redes pueden alcanzarse, y qué hacer al respecto. |
| `pkg/names` | Una clave convertida en un nombre que una persona puede decir. |
| `pkg/chat` | Líneas, tipeo y hablantes — la capa que usa la interfaz de este programa. |

**→ [ARCHITECTURE.es.md](ARCHITECTURE.es.md) recorre todo**: cómo se abre un
camino paso a paso, cómo se ve el formato en la red, cómo la malla se genera sus
propias claves, y una receta para construir encima. Diagramas, no prosa.

---

## Compilalo vos

La respuesta más corta a "¿debería confiar en este binario?":

```bash
git clone https://github.com/MalPr0/vapora && cd vapora
go build ./cmd/vapora
go test ./... -race
```

Go 1.25. Nada que descargar, nada que configurar.

Todas las declaraciones exportadas de `pkg/` están documentadas, y la
verificación está en el repo: `go run ./internal/doclint pkg`.

**La distribución, si querés leerla.** `pkg/punch` es el handshake, las sesiones
y las salas. `pkg/stun` averigua tu dirección y clasifica tu NAT. `pkg/upnp` y
`pkg/pcp` le piden a los routers que abran puertas. `pkg/dht` es el cliente de
BitTorrent. `pkg/diag` es el razonamiento detrás de los consejos. `internal/tui`
es el chat en pixel art.

[`ARCHITECTURE.es.md`](ARCHITECTURE.es.md) es el recorrido guiado de todo.
[`AGENTS.md`](AGENTS.md) documenta las invariantes — las cosas que parecen
detalles y terminan sosteniendo el edificio. Ese está solo en inglés, porque es
la referencia de trabajo del código.

---

<sup>Licencia MIT. Construido a la vista, un commit por vez.</sup>
