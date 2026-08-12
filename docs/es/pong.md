```
        █▀▀▀▄ ▄▀▀▀▄ █▄  █ ▄▀▀▀▄
        █▄▄▄▀ █   █ █▀▄ █ █  ▄▄
        █     █   █ █  ██ █   █
        ▀      ▀▀▀  ▀   ▀  ▀▀▀

     ▀▀▀ p o w e r e d   b y   v a p o r a ▀▀▀
```

[English](../../examples/pong/README.md) · **Español**

**Dos jugadores. Dos casas. Sin servidor.** Un tutorial de 200 líneas que
construye un juego real sobre `pkg/punch`, y de paso muestra el transporte usado
para algo que no se parece en nada a un chat.

← [volver al README](README.md) · [el recorrido del transporte](ARCHITECTURE.md)

---

**Contenido** · [Ejecutarlo](#ejecutarlo) · [La versión más chica](#paso-0--lo-más-chico-que-funciona) ·
[La idea](#la-idea-que-lo-hace-funcionar) · [Abrir un canal](#paso-1--abrir-un-canal) ·
[Quién tiene razón](#paso-2--decidir-quién-tiene-razón) · [Tu propio formato](#paso-3--definí-tu-propio-formato) ·
[No confiar en nada](#paso-4--no-confiar-en-lo-que-llega) · [El bucle](#paso-5--el-bucle) ·
[Decir qué pasa](#paso-6--decir-qué-está-haciendo-la-conexión) ·
[Qué demostró](#qué-demostró-esto) · [Llevalo a tu juego](#llevalo-a-tu-propio-juego)

---

## Ejecutarlo

```bash
go run ./examples/pong host            # imprime una invitación y espera
go run ./examples/pong join <invite>   # la otra máquina
```

`w`/`s` mueve, `r` empieza de nuevo, `q` sale. Gana el primero que llega a once.

```
      █▀▀▀▄ ▄▀▀▀▄ █▄  █ ▄▀▀▀▄
      █▄▄▄▀ █   █ █▀▄ █ █  ▄▄
      █     █   █ █  ██ █   █
      ▀      ▀▀▀  ▀   ▀  ▀▀▀

      powered by

      █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄
      █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █
      ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█
        ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀

      direct, encrypted, no server in the middle

      run this on the other machine:

        pong join 203.0.113.7:41001/BXFWOBXKGS547XF2WOKVG6JYDI

      waiting for a challenger...
```

Después la cancha, que es todo el juego:

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

---

## Paso 0 · Lo más chico que funciona

Antes del juego, el esqueleto. Esto es un programa completo: dos copias en dos
máquinas, en cualquier parte de internet, mandándose bytes.

```go
package main

import (
    "context"
    "fmt"
    "net"
    "os"
    "time"

    "github.com/MalPr0/vapora/pkg/punch"
    "github.com/MalPr0/vapora/pkg/stun"
)

func main() {
    ctx := context.Background()
    conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

    // El host acuña un secreto; el que se une lo saca de la invitación.
    secret, role := punch.Secret(nil), punch.RoleInviter
    var peer *net.UDPAddr
    if len(os.Args) > 1 {
        invite, _ := punch.ParseInvite(os.Args[1])
        secret, role, peer = invite.Secret, punch.RoleJoiner, invite.Endpoint
    } else {
        secret, _ = punch.NewSecret()
    }

    codec, _ := punch.NewSecretCodec(secret, role)

    mux := punch.NewMux(conn)
    watcher := stun.NewWatcher(stun.DefaultServers, stun.DefaultKeepalive)
    mux.Fallback(punch.SinkFunc(watcher.Handle))

    session := punch.NewSession(mux, codec, nil)
    mux.Fallback(session)
    if peer != nil {
        session.SetPeer(peer)
    }

    session.Observe(punch.ObserverFunc(func(payload []byte) {
        fmt.Println("←", string(payload))
    }))

    go mux.Run(ctx)
    go watcher.Run(ctx, conn)
    go session.Run(ctx)

    if peer == nil {
        endpoint, _ := watcher.Wait(ctx, 15*time.Second)
        fmt.Printf("run: go run . %s/%s\n", endpoint, secret)
    }

    if err := session.Open(ctx, 3*time.Minute); err != nil {
        fmt.Println("no path:", err)
        return
    }

    for range time.Tick(time.Second) {
        session.Send([]byte("hola"))
    }
}
```

Cuarenta líneas, sin dependencias, y la parte difícil — dos routers que rechazan
desconocidos — ya está resuelta. Todo lo que viene después es tu programa, no la
red.

---

## La idea que lo hace funcionar

Un chat y un juego quieren cosas opuestas del mismo canal.

```
   CHAT                            JUEGO
   ────                            ─────
   manda EVENTOS                   manda ESTADO
   importa cada uno                importa solo el último
   una línea perdida se perdió     un paquete perdido se corrige en 33ms
   necesita entrega                necesita frescura
```

Ese es todo el diseño. El host manda **el mundo entero** treinta veces por
segundo — pelota, paletas, marcador, once bytes — y el invitado dibuja lo que
llegó más recientemente. Nada se acusa, nada se retransmite, nada se ordena, y no
se pierde nada.

**Un paquete perdido en internet cuesta exactamente un cuadro.** El siguiente
trae la verdad completa igual.

A doce bytes por tick y treinta ticks por segundo, todo el juego usa **menos de
medio kilobyte por segundo** en cada dirección.

---

## Paso 1 · Abrir un canal

La mitad de red es corta como para leerla de una sentada. Un socket, un mux que
lo lee, un codec del secreto compartido, una sesión encima.

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)

mux := punch.NewMux(conn)
watcher := stun.NewWatcher(stun.DefaultServers, stun.DefaultKeepalive)
mux.Fallback(punch.SinkFunc(watcher.Handle))     // respuestas de STUN

session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)                            // lo que autentique

go mux.Run(ctx)          // lo único que lee el socket
go watcher.Run(ctx, conn)
go session.Run(ctx)
```

El host necesita su propia dirección antes de poder invitar a nadie:

```go
endpoint, _ := watcher.Wait(ctx, 15*time.Second)
fmt.Printf("pong join %s/%s\n", endpoint, secret)
```

Y después los dos lados perforan hasta que el camino se abre:

```go
session.Open(ctx, 3*time.Minute)
```

<sup>Eso es <a href="main.go">main.go</a>, y es la única parte de este ejemplo que tiene que ver con la red.</sup>

---

## Paso 2 · Decidir quién tiene razón

Alguien tiene que ser dueño de la pelota. Si las dos máquinas la simularan, se
irían separando en segundos — dos computadoras no pueden coincidir en física
sobre un enlace con pérdidas sin mucha maquinaria.

```
   HOST                                   INVITADO
   ────                                   ────────
   dueño de la pelota                     dueño de su propia paleta
   dueño de los dos marcadores            dueño de nada más
   simula cada tick            ────▶      dibuja lo que le dicen
                               ◀────      manda un número
```

La respuesta más vieja de los juegos en red, y todavía la correcta a esta escala.

---

## Paso 3 · Definí tu propio formato

El transporte te da **un** frame kind para tus bytes. Si necesitás más de un tipo
de mensaje, etiquetalos *adentro* del payload — así tu numeración y la del
transporte no pueden chocar nunca.

```go
const (
    tagPaddle byte = 1   // invitado → host
    tagState  byte = 2   // host  → invitado
    tagReset  byte = 3   // invitado → host: empezar de nuevo
)
```

El reset muestra el diseño rindiendo. El invitado no puede resetear nada — solo
el host simula — así que manda un byte pidiéndolo, y después se entera del
marcador nuevo por el siguiente estado, igual que se entera de todo lo demás. Sin
acuse, sin caso especial, y sin manera de que los dos lados discrepen sobre si
pasó.

El mundo entero son once bytes, doce con la etiqueta adelante:

```
   tag  │            el juego completo, en cada tick
  ──────┼──────────────────────────────────────────────────────
   1 B  │ ballX  ballY  leftY  rightY  marcadores  serving
        │  2 B    2 B    2 B    2 B       2 B        1 B    = 11
```

Las posiciones viajan como fracción de un campo fijo, no como celdas de terminal,
así los dos jugadores pueden tener ventanas de distinto tamaño y coincidir igual
sobre el juego.

```go
const fieldWidth, fieldHeight = 1000, 1000
```

<sup><a href="wire.go">wire.go</a></sup>

---

## Paso 4 · No confiar en lo que llega

Todo lo que viene de la red es una afirmación. El transporte garantiza que vino
de alguien con el secreto — **nada más que eso**.

```go
func decodeState(payload []byte) (State, bool) {
    if len(payload) != 1+stateBytes || payload[0] != tagState {
        return State{}, false          // no es este programa del otro lado
    }
    return State{
        BallX: clamp16(binary.BigEndian.Uint16(payload[1:]), fieldWidth),
        ...
    }, true
}
```

Dos reglas, y son las mismas en todos lados:

- **Un largo que no coincide no es tu programa.** Descartalo; no adivines.
- **Acotá cualquier cosa que vaya a indexar algo.** Un par que afirme que la
  pelota está en 65535 estaría escribiendo más allá del final de tu buffer de
  pantalla.

---

## Paso 5 · El bucle

Idéntico en las dos máquinas. Lo único que cambia es de qué es dueño cada uno.

```go
select {
case key := <-pressed:
    switch key {
    case 'w', 'k': world.move(me, -paddleSpeed)   // mi paleta, siempre mía
    case 's', 'j': world.move(me, paddleSpeed)
    }

case payload := <-t.incoming:
    if hosting {
        if y, ok := decodePaddle(payload); ok {
            world.paddle[1] = y             // lo único que le creo
        }
    } else if received, ok := decodeState(payload); ok {
        state = received                    // todo, reemplazado entero
    }

case <-ticker.C:                            // 33ms
    if hosting {
        world.tick()
        t.session.Send(encodeState(world.state))
    } else {
        t.session.Send(encodePaddle(world.paddle[1]))
    }
    display.draw(state)
    fmt.Print(display.render(state, t.me, t.them, t.status(world, hosting)))
}
```

**Nunca bloquees el transporte.** Entrega en su propia goroutine, así que el
observador pasa a un canal y descarta cuando el juego se queda atrás — un cuadro
viejo no vale nada, y una cola de cuadros viejos vale menos:

```go
session.Observe(punch.ObserverFunc(func(payload []byte) {
    select {
    case built.incoming <- payload:
    default:                    // ¿atrasado? el más nuevo llega en 33ms
    }
}))
```

---

## Paso 6 · Decir qué está haciendo la conexión

Un juego entre dos casas va a tener un mal minuto. Si simplemente se congela, el
jugador le echa la culpa a tu juego.

```go
health := t.session.Health()

switch health.Link {
case punch.LinkLost:  return "connection lost"
case punch.LinkStale: return fmt.Sprintf("no reply for %ds", int(health.Silence.Seconds()))
default:              return fmt.Sprintf("%dms", health.RTT.Milliseconds())
}
```

El transporte mide el camino por vos, con un ping relleno cuya respuesta se
rellena por separado — así ninguno de los dos es reconocible por su tamaño para
alguien que esté mirando.

---

## Qué demostró esto

Los tests que están al lado de este archivo son el punto del ejercicio:

| | |
|---|---|
| `TestTheBallReachesTheOtherSide` | 60 ticks, y la pelota aparece en una docena de lugares — el estado cruza de verdad |
| `TestThePaddleReachesTheHost` | la única autoridad del invitado llega intacta |
| `TestALostPacketCostsNothing` | un estado sobrevive entero a su propia codificación, y por eso nada necesita retransmitirse |
| `TestNonsenseFromTheNetworkIsRefused` | largos incorrectos descartados, posiciones imposibles acotadas |
| `TestATickIsSmall` | once bytes, que es la razón por la que 30 por segundo no es nada |
| `TestEitherSideCanAskToStartAgain` | el invitado pide, el host decide, el marcador nuevo llega como estado común |

Arman sus sesiones solo con la API exportada, exactamente como lo hace el juego.
Si un juego no se pudiera armar desde afuera del transporte, la separación en
capas sería una afirmación en vez de un hecho.

```bash
go test ./examples/pong/ -race
```

---

## Lo que enseñó jugarlo

Tres partidas completas entre dos personas, y cada lección fue sobre el juego, no
sobre el canal.

**Una paleta quieta le ganó a una persona 7-6.** La primera versión tenía una
paleta que cubría el 16% de la cancha y una pelota que casi no cambiaba de
ángulo. Nada estaba roto; simplemente no había dificultad. Pelota más lenta, más
chica, y once puntos para ganar.

**Perseguir la pelota pierde contra anticiparla.** Un lado lo jugó un script que
leía la pantalla cada 120ms y se movía hacia donde la pelota estaba en ese
momento. A 12 unidades por tick y 30 ticks por segundo, la pelota recorre **43
unidades entre una mirada y la siguiente**, así que la paleta apuntaba
permanentemente a donde la pelota había estado. Se fue 4-1 arriba y perdió 4-11
apenas la otra persona empezó a usar ángulos.

Ese es el mismo problema que tiene todo juego en red, en miniatura. Es la razón
por la que los juegos reales **predicen** en vez de seguir, y es para lo que
sirven las piezas faltantes de abajo.

---

## Qué falta, honestamente

Esto es un tutorial, no un juego terminado. Nada de esto es sobre el canal.

| Falta | Qué haría falta |
|---|---|
| **Interpolación** | A 30 ticks la pelota da saltos en vez de deslizarse. Dibujar entre los dos últimos estados en lugar de sobre el más nuevo. |
| **Predicción** | La paleta del invitado espera una ida y vuelta; arriba de 200ms se siente. Moverse localmente al instante y reconciliar cuando llega la versión del host. |
| **Una autoridad justa** | El host no puede perder por lag y el invitado no puede ganar por eso. Está bien entre amigos, no para nada competitivo. |
| **Windows** | La mitad de `pkg/` es portable. termios no. |

---

## Llevalo a tu propio juego

Las partes de esto que no son sobre Pong:

- **Mandá estado, no eventos**, siempre que el mensaje más nuevo vuelva
  irrelevantes a los anteriores. Te compra inmunidad a la pérdida de paquetes
  gratis.
- **Poné tus etiquetas adentro del payload.** El transporte te da un frame kind;
  tu numeración vive debajo de él y no puede chocar con nada.
- **Elegí un solo dueño por cada hecho.** La pelota tiene exactamente uno, la
  paleta del invitado tiene exactamente uno. Cualquier cosa con dos dueños va a
  discrepar, y ahí estás escribiendo un algoritmo de consenso en vez de un juego.
- **Acotá y limitá todo lo que llega.** El transporte prueba quién lo mandó,
  nunca que tenga sentido.
- **Nunca bloquees en el observador.** Pasalo a un canal y descartá cuando se
  llena: con estado, el más nuevo llega enseguida y la cola no vale nada.
- **Mostrá la conexión.** `session.Health()` te da ida y vuelta y silencio. A un
  juego que se congela sin decir por qué le echan la culpa de la red.

---

## Los archivos

| | |
|---|---|
| [`main.go`](../../examples/pong/main.go) | La configuración de red, y la única parte sobre internet |
| [`wire.go`](../../examples/pong/wire.go) | El protocolo: tres etiquetas y once bytes |
| [`game.go`](../../examples/pong/game.go) | Las reglas, que corren solo en el host |
| [`play.go`](../../examples/pong/play.go) | El bucle, idéntico en los dos lados |
| [`screen.go`](../../examples/pong/screen.go) | Dibujo con medio bloque |
| [`splash.go`](../../examples/pong/splash.go) | Los wordmarks |
| [`keys.go`](../../examples/pong/keys.go) | Modo raw, unas treinta líneas de termios |
| [`pong_test.go`](../../examples/pong/pong_test.go) | Las verificaciones de la tabla de arriba |

---

<sup>No entra en los releases — lo que se compila es <code>cmd/vapora</code>. Esto vive acá para ser leído.</sup>
