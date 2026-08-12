```
██      ██      ██      ████████      ██████    ████████        ██
██      ██    ██  ██    ██      ██  ██      ██  ██      ██    ██  ██
██      ██  ██      ██  ██      ██  ██      ██  ██      ██  ██      ██
██      ██  ██      ██  ████████    ██      ██  ████████    ██      ██
██      ██  ██████████  ██          ██      ██  ██    ██    ██████████
  ██  ██    ██      ██  ██          ██      ██  ██      ██  ██      ██
    ██      ██      ██  ██            ██████    ██      ██  ██      ██
```

[English](../../README.md) · [Español](../es/README.md) · [中文](../zh/README.md) · [日本語](../ja/README.md) · **Português** · [العربية](../ar/README.md) · [Français](../fr/README.md) · [Italiano](../it/README.md) · [Deutsch](../de/README.md) · [Русский](../ru/README.md)

### Converse direto do seu computador para o da outra pessoa. Sem servidor. Sem conta. Sem rastro.

Você compartilha uma linha de texto. A outra pessoa cola. Vocês já estão
conversando — criptografado, direto, sem nada no meio.

[![release](https://img.shields.io/github/v/release/MalPr0/vapora?style=flat-square&color=e8a33d)](https://github.com/MalPr0/vapora/releases/latest)
![go](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square)
![dependências](https://img.shields.io/badge/dependências-zero-2ea043?style=flat-square)
![licença](https://img.shields.io/badge/licença-MIT-blue?style=flat-square)

---

## Experimente em 30 segundos

```bash
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz
./vapora punch
```

Ele imprime uma linha. Mande para alguém. Essa pessoa cola no terminal dela.

<sup>Outras versões: `darwin_amd64` · `linux_amd64` · `linux_arm64` · `windows_amd64.zip` — troque o nome na URL. Use `curl`, não o navegador: um navegador marca o que baixa como não confiável e o macOS depois se recusa a executar.</sup>

---

## Como é na prática

```
 █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄                    ● JADE HERON     31ms
 █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █                   ● SWIFT OTTER    47ms
 ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█                   ◐ GREY MARTEN  no reply 9s
   ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ you are CRIMSON QUAIL ━━━━━━━━━━━━━━━━━━━━━━━━━

  --             JADE HERON joined
  JADE HERON     tem alguém aí?
  SWIFT OTTER    @QUAIL olha isso
▸ CRIMSON QUAIL  já vou
  GREY MARTEN    ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
> hola_
                        enter sends · pgup/pgdn scrolls · !exit quits
```

Um chat de terminal em pixel art retrô. Cada pessoa recebe um nome de animal que
ninguém pode reivindicar, as `@menções` puxam uma linha para fora da rolagem, e
um corredorzinho atravessa a tela de carregamento enquanto a conexão abre
caminho.

---

## Por que isso pode te servir

**Ninguém está no meio.** Suas palavras vão da sua máquina para a da outra
pessoa. Não passam pelos servidores de nenhuma empresa nem pelos meus. Não há um
meio para intimar, vender ou invadir.

**Não há nada para se cadastrar.** Sem e-mail, sem telefone, sem usuário, sem
perfil. O programa não sabe quem você é, e ninguém mais sabe.

**Nada é armazenado.** Feche e a conversa some das duas pontas. Não há histórico
para vazar, porque não existe histórico.

**Um arquivo, zero dependências.** Baixe um binário e execute. Sem Docker, sem
runtime, sem instalação. Construído com a biblioteca padrão do Go e mais nada —
você pode ler cada linha do que é distribuído.

**Criptografado por padrão, sem como desligar.** AES-256-GCM, uma chave
diferente para cada direção. O convite que você compartilha *é* a chave.

**Grupos são uma malha de verdade.** Todo mundo fala com todo mundo diretamente.
Duas pessoas numa sala de cinco têm um canal que as outras três não conseguem
ler — não como promessa de comportamento, mas como aritmética: elas não têm as
chaves.

---

## Para que as pessoas usam

- **Mandar algo sensível** para um colega sem que aquilo viva para sempre no log
  de chat de uma empresa.
- **Conversar através de um firewall** onde você não pode abrir portas nem
  instalar nada.
- **Um canal rápido com alguém** que não deixa conta, nem histórico, nem rastro
  em nenhuma das duas máquinas.
- **Entender a sua própria conexão** — o diagnóstico te conta mais sobre a sua
  rede do que o seu provedor.

---

## Duas pessoas

```bash
./vapora punch                 # você: imprime um convite
./vapora punch "<o convite>"   # a outra pessoa: cola e executa
```

**Se não conectar, mandem um convite cada um.** Roteadores domésticos costumam
recusar pacotes de desconhecidos, então quando os dois lados fazem isso, o
primeiro pacote de cada um morre na porta do outro. A tela da outra pessoa
imprime uma linha embaixo de *"if it does not connect, send this back"* — peça
que mande, cole no seu terminal, e agora os dois estão batendo na porta no mesmo
instante. É exatamente isso que esses roteadores precisam ver.

Dá para descobrir antes se você vai precisar desse passo — veja
[diagnóstico](#conheça-sua-rede-antes-de-culpá-la).

## Um grupo

```bash
./vapora room                  # abre uma sala e imprime um convite
./vapora room "<o convite>"    # qualquer pessoa entra com ele
```

**Qualquer um pode convidar.** Entrou há cinco minutos? `!invite` te dá uma linha
para trazer a próxima pessoa. Todo mundo acaba conhecendo todo mundo sem voltar
a quem abriu a sala.

**Quem te convidou não é um servidor.** Apresenta duas pessoas e sai da frente.
Não carrega nada entre elas e não conseguiria ler nem se quisesse. Desligue a
máquina que abriu a sala e a conversa continua sem ela.

**Salas comportam oito**, e **fecham quando esvaziam** — uma sala sem ninguém é
uma porta sem dono. Use `-standalone` se quiser que uma fique esperando.

**Vocês dois no mesmo wifi?** Também funciona. Cada participante anuncia o
endereço público e o local, porque duas máquinas atrás do mesmo roteador não
conseguem se alcançar pelo público. Se resolve sozinho em alguns segundos.

### Enquanto você está dentro

| | |
|---|---|
| `@nome` | puxa a sua linha para fora da rolagem da outra pessoa, com uma marca na margem |
| `!who` | quem está presente, e a saúde de cada conexão |
| `!invite` | um convite novo para trazer alguém |
| `!exit` | sair, avisando todo mundo na hora |
| `PgUp` / `PgDn` | voltar no que foi dito |
| `-plain` | linhas simples em vez da tela cheia, para quando algo dá errado |

---

## Como funciona

Seu computador não tem endereço próprio na internet. Quem tem é o seu roteador, e
tudo na sua casa divide esse endereço. Isso é **NAT**, e é por isso que ninguém
consegue simplesmente "ligar" para o seu notebook. A resposta usual é colocar um
servidor no meio ao qual os dois lados se conectam *para fora* — funciona, e
significa que o computador de outra pessoa vê cada palavra.

O vapora faz outra coisa. Os dois lados mandam pacotes *para fora* no mesmo
instante, cada um furando um buraco no próprio roteador, e os dois buracos se
alinham. Depois disso o caminho é direto e não há mais ninguém nele.

| O quê | Por que está aqui |
|---|---|
| **UDP hole punching** | O caminho direto em si. Os dois lados furam ao mesmo tempo e se encontram no meio. |
| **STUN** ([5389](https://www.rfc-editor.org/rfc/rfc5389), [5780](https://www.rfc-editor.org/rfc/rfc5780)) | Descobre qual endereço o mundo externo enxerga, e classifica o comportamento do seu roteador. |
| **UPnP-IGD, PCP, NAT-PMP** | Três idiomas para pedir a um roteador que abra uma porta. Tenta os três, porque roteadores raramente concordam sobre qual falam. |
| **X25519 + HKDF + AES-256-GCM** | Uma chave separada por par e por direção. Numa sala, nenhum membro lê o tráfego de outro par. |
| **Janela anti-replay** | Janela deslizante no estilo IPsec, por remetente, para que um pacote capturado não possa ser reproduzido contra você. |
| **DHT do BitTorrent** *(opcional)* | Encontrar-se sem endereço nenhum. Desligado por padrão — veja [segurança](#segurança). |

Tudo da biblioteca padrão do Go. Nenhum código de terceiros, em lugar nenhum.

<sup><a href="../../ARCHITECTURE.md">ARCHITECTURE.md</a> tem o passo a passo, com diagramas.</sup>

---

## Conheça sua rede antes de culpá-la

```bash
./vapora nat                   # que tipo de roteador você tem na frente
./vapora diag                  # cada roteador entre você e a internet
```

`nat` imprime um perfil curto tipo `CONE-PORT-18`. Mande para quem você quer
conectar, coloque o dela, e ele te diz o que esperar **antes** de você perder uma
noite:

```bash
./vapora nat -pair CONE-OPEN-64                    # para duas pessoas
./vapora nat -room "CONE-PORT-18,SYM-PORT-F3"      # para uma sala inteira
```

Se uma conexão funciona é uma propriedade do *par*, não de nenhum dos lados —
nenhuma medição da sua própria rede responde isso sozinha. É por isso que o
perfil foi feito para ser colado para outra pessoa. Para uma sala vai além: diz
se a malha fecha, quem deveria hospedar, e exatamente qual par nunca vai se
alcançar.

<sup>Se um firewall abre uma porta específica, meça aquela: <code>vapora nat -port 41000</code>. Filtragem é propriedade de uma porta, não de uma máquina.</sup>

---

## Segurança

**O convite é a chave.** Aquela string não é um endereço, é o segredo que
criptografa tudo. Trate como senha: qualquer um que a veja — num print, num grupo
de chat, por cima do ombro — pode usá-la.

**Silêncio para desconhecidos.** Pacotes sem a chave certa não recebem resposta
alguma. Um scanner de portas aprende exatamente o que aprenderia de uma porta
fechada. Mas eles são contados, e **você é avisado**, porque significa que
alguém achou um endereço que só deveria ter estado em um convite.

**Ninguém toma sua conversa.** Passe seu convite para uma terceira pessoa e
mesmo assim ela não consegue expulsar seu amigo. O programa distingue os dois,
ignora o recém-chegado e te avisa.

**Só texto atravessa.** Qualquer outra coisa é descartada em vez de exibida. E do
texto que vem da rede são removidas as sequências de escape que permitiriam a
alguém mover seu cursor, repintar sua tela ou alcançar sua área de transferência.

**Um convite continua válido até você fechar o programa.** Não expira e não há
como revogar. Fechar e reabrir *é* a revogação — te dá uma chave nova e
normalmente um endereço novo.

**Numa sala, um membro pode mentir sobre quem mais está.** Pode anunciar alguém
que não existe. O que não pode é ler nem forjar o que duas outras pessoas dizem.
Um membro inventado nunca responde e cai sozinho.

**"Sem conta" não é o mesmo que invisível.** A pessoa com quem você fala vê seu
endereço IP. Tem que ver — os pacotes vão da sua casa para a dela. É isso que
*direto* significa, e é a troca honesta por não ter servidor.

**`-discover` publica seu endereço numa rede pública**, e por isso vem desligado.
Com ele, os dois lados se encontram pelo DHT do BitTorrent sob um nome derivado
do seu segredo. Ninguém consegue te procurar sem esse segredo, mas você vira mais
uma linha numa tabela que qualquer um pode varrer.

---

## O que vai quebrar, e quando

Limitações honestas, não letra miúda.

- **Os servidores STUN são de outros** — Google, Cloudflare e mais dois, serviços
  gratuitos que existem para outra coisa. Se sumirem, isto não consegue descobrir
  o próprio endereço, e hoje não há alternativa.
- **Algumas redes bloqueiam direto**: empresas, universidades, hotéis, algumas
  operadoras móveis. Nada do seu lado resolve.
- **Algumas conexões simplesmente não conseguem.** Um NAT *simétrico* ou de
  operadora deixa seu endereço imprevisível de um momento para outro, então não
  há para onde mirar. `vapora nat` te avisa. A única solução é um relay, que isto
  deliberadamente não tem.
- **Seu endereço muda e o convite morre.** Trocar de wifi, ir para dados móveis,
  ficar parado tempo suficiente. O programa percebe e imprime um novo, mas você
  tem que mandar de novo.
- **As versões precisam bater.** O formato já mudou várias vezes e vai mudar de
  novo. Versão velha e nova não se entendem, e o sintoma é *silêncio*. Rodem
  `./vapora version` os dois antes.
- **Nada é protegido retroativamente.** Alguém que grave seu tráfego hoje e
  consiga seu convite depois consegue ler aquela gravação. Ferramentas sérias
  resolvem isso com chaves descartadas ao longo do caminho. Esta não.
- **Os binários não são assinados.** Seu sistema vai avisar, e faz bem. Verifique
  o checksum contra `SHA256SUMS`, ou compile você mesmo.
- **`vapora serve` muda a configuração do seu roteador.** É a demo original de
  UPnP, e o único comando aqui que pede ao roteador para abrir uma porta para a
  internet. Ele fecha ao sair — mas se travar, essa porta pode ficar aberta até
  o roteador reiniciar. Todo o resto deste README não toca no seu roteador.
- **Ninguém que quebra software profissionalmente revisou isto.** Ser construído
  com cuidado não é o mesmo que ser auditado. Não aposte nada que importe.

---

## Como você pode usar isto

<sup>O <code>ARCHITECTURE</code> e o tutorial do Pong, linkados abaixo, por enquanto só existem em inglês.</sup>

O chat é uma coisa construída sobre o canal, não o objetivo dele. O transporte é
uma camada separada que não faz ideia do que é uma conversa: abre um caminho
criptografado através de dois roteadores, mantém uma malha viva, e move **bytes**.

Quarenta linhas já são um programa que funciona — duas cópias dele, em duas
máquinas em qualquer lugar da internet, trocando bytes sem nada no meio:

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
mux := punch.NewMux(conn)
session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)

session.Observe(punch.ObserverFunc(func(payload []byte) {
    fmt.Println("←", string(payload))       // exatamente o que mandaram
}))

go mux.Run(ctx)
go session.Run(ctx)

session.Open(ctx, 3*time.Minute)             // fura os dois roteadores
session.Send([]byte("hola"))
```

### 🏓 Comece por aqui: [**construa um Pong**](../../examples/pong/README.md)

Um tutorial passo a passo que vai daquele esqueleto até um jogo real de dois
jogadores pela internet — seu próprio formato na rede, quem tem o direito de
estar certo sobre o quê, e por que um jogo sobrevive a uma perda de pacotes que
arruinaria uma conversa.

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

### Três coisas sobre o mesmo canal

| | Manda | Se importa com |
|---|---|---|
| **[Pong](../../examples/pong/README.md)** — tutorial | **estado**, 30 vezes por segundo | só o mais novo. Um pacote perdido custa um quadro |
| **[filedrop](../../examples/filedrop)** | **blocos** de um arquivo | todos, e no lugar certo |
| **`vapora punch` / `room`** | **eventos** — linhas de texto | cada uma delas |

Um jogo e uma conversa querem coisas opostas do mesmo transporte — frescor contra
entrega — e nenhum dos dois precisou que o transporte mudasse. Essa é a evidência
mais clara de que a separação em camadas é real, e é por isso que construir em
cima não significa herdar as decisões de outra pessoa.

### Os pacotes

| Pacote | O que te dá |
|---|---|
| `pkg/punch` | O caminho, a criptografia, a malha. Bytes entram, bytes saem. |
| `pkg/stun` | Seu endereço público, e uma classificação do seu NAT. |
| `pkg/upnp`, `pkg/pcp` | Pedir a um roteador que abra uma porta, em três protocolos. |
| `pkg/dht` | Anunciar e encontrar um endereço no DHT do BitTorrent. |
| `pkg/diag` | Se duas redes conseguem se alcançar, e o que fazer. |
| `pkg/names` | Uma chave transformada num nome que uma pessoa consegue falar. |
| `pkg/chat` | Linhas, digitação e falantes — a camada que a interface deste programa usa. |

**→ [ARCHITECTURE.md](../../ARCHITECTURE.md) percorre tudo**: como um caminho é aberto
passo a passo, como é o formato na rede, como a malha gera as próprias chaves, e
uma receita para construir em cima. Diagramas, não prosa.

---

## Compile você mesmo

A resposta mais curta para "devo confiar neste binário?":

```bash
git clone https://github.com/MalPr0/vapora && cd vapora
go build ./cmd/vapora
go test ./... -race
```

Go 1.25. Nada para baixar, nada para configurar.

Toda declaração exportada em `pkg/` está documentada, e a verificação está no
repositório: `go run ./internal/doclint pkg`.

**A organização, se você quiser ler.** `pkg/punch` é o handshake, as sessões e as
salas. `pkg/stun` descobre seu endereço e classifica seu NAT. `pkg/upnp` e
`pkg/pcp` pedem aos roteadores que abram portas. `pkg/dht` é o cliente
BitTorrent. `pkg/diag` é o raciocínio por trás dos conselhos. `internal/tui` é o
chat em pixel art.

[`ARCHITECTURE.md`](../../ARCHITECTURE.md) é o passeio guiado por tudo isso.
[`AGENTS.md`](../../AGENTS.md) documenta as invariantes — as coisas que parecem
detalhe e acabam sustentando o prédio. Esse está só em inglês, porque é a
referência de trabalho do código.

---

<sup>Licença MIT. Construído à vista, um commit por vez.</sup>
