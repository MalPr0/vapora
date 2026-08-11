# vapora

Two people, or a few, talking directly to each other across the internet. No
account, no registration, no server in the middle. You share one line of text
and the other person runs it.

The chat you get is the proof that it works, not the point. The point is the
channel underneath it.

---

## 1. Installing it

Every push to this project publishes ready-to-run programs. You do not need Go,
a compiler, or anything else.

**Download it with `curl`, from a terminal. Not with your browser.** This
matters on macOS: a browser marks whatever it downloads as untrusted, and macOS
then refuses to run it because Apple has not signed off on this program. `curl`
does not add that mark, so it just runs.

```bash
# macOS on Apple Silicon (M1 and newer)
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz

# macOS on Intel
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_amd64.tar.gz | tar -xz

# Linux
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_linux_amd64.tar.gz | tar -xz
```

That leaves a file called `vapora` in the folder you are standing in. Check it
runs:

```bash
./vapora version
```

On Windows, download `vapora_windows_amd64.zip` from the releases page and
unzip it.

### Both of you need the same version

This program talks to itself in a format that has changed and will change
again. **If two people run different versions, nothing happens and nothing
explains why.** Before anything else, both run `./vapora version` and compare.

### Checking what you downloaded

This program opens a door in your network, so it is worth a minute to confirm
you got the real thing and not something swapped in transit.

```bash
curl -fsSLO https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz
curl -fsSLO https://github.com/MalPr0/vapora/releases/latest/download/SHA256SUMS

shasum -a 256 -c SHA256SUMS --ignore-missing   # macOS
sha256sum -c SHA256SUMS --ignore-missing       # Linux
```

`OK` means the file is byte for byte what was published.

### If you already downloaded it with a browser

macOS will refuse to open it and say Apple could not verify it. That warning is
honest: these files are not signed with an Apple developer certificate, so Apple
really has not checked anything. After confirming the checksum above, you can
clear the mark yourself:

```bash
xattr -d com.apple.quarantine ./vapora
```

Downloading with `curl` avoids the whole thing.

### Or build it yourself

If you have Go installed, this answers the trust question by not asking it:

```bash
go build ./cmd/vapora
```

---

## 2. Talking to one person

One of you goes first and the other joins.

**You:**

```bash
./vapora punch
```

It prints a line like this:

```
vapora punch 203.0.113.7:41001/BXFWOBXKGS547XF2WOKVG6JYDI
```

Send that to your friend however you normally talk — it is just text.

**Your friend:** pastes the whole line into their terminal and runs it.

If the two of you are lucky, that is it. Often it is not, and there is one more
step.

### The step most people miss

Home routers usually refuse packets from a stranger they have never sent
anything to. When both of your routers do that, your friend's first packet dies
at your door and yours dies at theirs. Neither of you did anything wrong.

The fix is that **both of you send an invite**, not just one:

1. You run `./vapora punch` and send your line.
2. Your friend pastes it and runs it.
3. **Their screen now prints a line too**, under the words *"if it does not
   connect, send this back"*. They send that one to you.
4. You paste it into your terminal, which is still waiting.

Now both of you are knocking on each other's door at the same moment, which is
exactly what those routers need to see. This is normal, not a workaround.

Section 6 shows how to find out in advance whether you need this step.

### While you are chatting

Type and press enter. Some things worth knowing:

- **`@name`** marks a line as addressed to someone. Their screen pulls it out
  of the conversation with a mark in the margin, so it is still findable after
  it scrolls past.
- **`!exit`** leaves, and tells the other side right away instead of leaving
  them staring at a frozen screen.
- **Page Up and Page Down** scroll back through what was said.
- **`-plain`** turns off the full-screen interface and prints plain lines
  instead. Use it when something is wrong: the full-screen version erases
  itself when it closes, and plain output stays in your terminal where you can
  read it or send it to someone.

---

## 3. Talking to several people

A room is a separate command. It is not the two-person chat with more chairs:
the way people are protected from each other has to work differently once there
are three, so it has its own handshake and its own invite.

**Whoever starts:**

```bash
./vapora room
```

It prints an invite. Anyone who runs it joins.

**Everyone else:**

```bash
./vapora room "vapora room 203.0.113.7:41001/AE3LG7ILJPAT..."
```

### What is different about a room

**Everyone talks to everyone directly.** The person who invited you does not
carry your words to anyone. They introduce you and step out of the way, and even
if they wanted to, they could not read what you say to a third person.

**Anyone can invite.** Not just whoever started it. If you joined five minutes
ago, `!invite` gives you a line to bring in the next person, and everybody ends
up knowing everybody without asking the original host.

**Names are chosen for you.** You will be something like `OTTER` or, if two
people would otherwise clash, `CRIMSON OTTER`. Nobody picks their own name and
everybody sees the same names for the same people, so a name always means one
specific person and cannot be claimed by somebody else.

**Rooms hold eight.** Past that it is a lot of connections for home routers to
keep open, and quality falls off faster than the conversation improves.

**A room looks like the two-person chat, with a list.** Everyone present is
shown down the right-hand side, each with their own connection health, because
in a room the connections are separate: one person's can go bad while everyone
else stays fine. The typing line names who is writing, and `@` highlighting
works the same way it does with two.

While you are the one waiting for the first person, the screen keeps showing
your invite — that is the whole reason to be waiting. Once somebody arrives the
chat takes over, and `!invite` brings the invite back whenever you need it.

### Room commands

- **`!who`** lists who is present and whether each connection is healthy.
- **`!invite`** prints a fresh invite to bring somebody in.
- **`!exit`** leaves and tells everyone.
- **`-plain`** turns off the full-screen interface here too, for the same
  reason: when something goes wrong you want the output to stay on screen.

---

## 4. What it actually does

Some background in plain terms, then the specifics.

### The problem

Your computer does not have its own address on the internet. Your router has
one, and everything in your home shares it. That is called **NAT**. It is why
someone cannot simply "call" your laptop: from the outside, your laptop is not
addressable.

The usual answer is to put a server in the middle that both sides connect *out*
to. That works, and it means somebody else's computer sees every word.

### What this does instead

**Hole punching.** When your computer sends a packet out, your router briefly
remembers "if something comes back this way, it belongs to them". If both
computers send to each other at the same moment, both routers open that memory
at the same time, and the two sides meet in the middle. No server, and the
conversation goes straight from one house to the other.

For that you need to know your own address as the outside world sees it, which
is what **STUN** is for: a small public service that answers "this is where your
packet came from". It sees your address; it never sees your conversation.

### The specifics

Everything below is written from scratch against the Go standard library. There
are no third-party dependencies at all.

**Finding your address and understanding your router**

- **STUN** (RFC 5389) to learn the address the internet sees for you.
- **NAT behaviour discovery** (RFC 5780) to find out what your router allows.
- **UPnP-IGD** (SSDP and SOAP) to ask your router to open a door, when it will.
- **PCP** (RFC 6887) and **NAT-PMP** (RFC 6886), two other ways to ask, for the
  routers that refuse the first one.

**Protecting the conversation**

- **X25519** key agreement (`crypto/ecdh`) so each pair of people in a room
  derives a channel only those two can read.
- **HKDF-SHA256** to turn that agreement into keys.
- **AES-256-GCM** on every single packet, with a separate key per direction.
- A **replay window** in the style of IPsec, so a packet captured and sent again
  later is rejected.
- **Random padding** on the packets that carry no content, so someone watching
  the network cannot pick out the heartbeats by their size and fingerprint what
  you are running.

**Keeping it alive**

- A ping every five seconds that also keeps the router's memory fresh.
- Twelve seconds of silence reads as trouble, forty-five as lost, and it keeps
  trying either way.
- If the other person's address changes, the channel follows them, but only if
  it is really them (see section 7).

---

## 5. Why this will stop working someday

Honest limitations, not fine print.

**The public STUN servers are somebody else's.** This program asks Google,
Cloudflare and two others where your packets come from. They are free services
run for other purposes. If they disappear, move, or start blocking, this program
cannot learn your own address and cannot print an invite. There is no fallback
today.

**Some networks block it outright.** Company networks, universities, hotels and
some mobile carriers filter this kind of traffic. Nothing on your end fixes
that.

**Some home connections cannot do it at all.** If your provider gives you what
is called a *symmetric* NAT, or puts you behind their own large shared NAT
(carrier-grade NAT, common on mobile), your address is unpredictable from one
moment to the next and there is nothing to aim at. `vapora nat` tells you if you
are in this situation. There is no software fix: it needs a relay server, which
this program deliberately does not have.

**Your address changes and the invite dies.** The line you shared points at a
specific door. Change wifi, switch to mobile data, or let it sit idle long
enough and that door is gone. The program notices and prints you a new invite,
but you have to send it again.

**Versions stop matching.** The format two copies of this program use to talk
has changed several times and will change again. Old and new do not
interoperate, and the symptom is silence.

**Nobody can join a conversation that already ended.** There is nothing stored
anywhere. Close the program and the conversation is gone, on both sides.

**One thing that would help does not exist yet.** For two people who both have
restrictive routers, a single invite is not enough (section 2). Fixing that
needs a way for two strangers to find each other, which normally means a server.
There is a design for doing it without one, using the BitTorrent network as a
meeting point, and it is not built.

---

## 6. Finding out what is wrong

### Will this work at all?

```bash
./vapora nat
```

This measures your connection and finishes with a short code:

```
your profile: CONE-PORT-18
```

That code describes your side. **It cannot tell you whether a connection will
work**, because that depends on both networks, not just yours. So send it to the
person you want to talk to, get theirs, and put them together:

```bash
./vapora nat -pair CONE-OPEN-64
```

Now you get a real answer: whether a direct connection is possible at all,
whether one invite is enough or both of you need to exchange one, and **which of
you has to be the one waiting**. That last part matters more than it sounds — if
you both wait for each other, you both wait forever, and it looks exactly like a
broken network.

One result is bad news on its own: if your code starts with `SYM`, your provider
gives you an unpredictable address and no amount of trying will help.

### A closer look at your routers

```bash
./vapora diag
```

This checks each router between you and the internet, asks each one in three
different languages whether it will open a door, and — if you have more than one
router, which is common — runs an experiment to work out **which one** is the
problem. That distinction is what tells you whether it is something you can fix
or something at your provider.

It works even on a network with no cooperative router at all; it just says so
and carries on with the rest.

### When a connection failed

Run with **`-plain`** — it works on both `punch` and `room`. The full-screen
chat erases itself when it closes, so a failed attempt leaves you with an empty
screen and no clue. Plain mode leaves everything in your terminal.

Either way, when a full-screen session ends without connecting it prints why,
and the invite it was offering, after the screen is handed back — so a failed
attempt is still something you can paste to someone.

Closing the full-screen version also prints a short summary of what happened:
whether a connection was ever made, why not, the invite you were offering, and
whether you were the one waiting.

### Other tools

- **`./vapora version`** — the first thing to compare when two people cannot
  connect.
- **`./vapora probe`** — shows your router and the address it thinks it has.

---

## 7. Security and privacy

### What protects the conversation

**The invite is the key.** That string at the end of the line you share is not a
name or an address — it is the secret that encrypts the conversation. Everything
sent is encrypted with it using AES-256-GCM, with a different key for each
direction. Someone who watches your network sees packets and learns nothing from
them.

**There is no unencrypted mode.** It cannot be turned off, and there is no
option to. A program that opens a door to the internet has no business running
without it, and the encryption costs nothing.

**In a room, each pair has its own key.** Two people in a room of five have a
channel that the other three cannot read, even though everyone holds the same
invite. This is not a promise about behaviour, it is arithmetic: the other three
do not have the keys.

**Someone else cannot take over your conversation.** If you hand your invite to
a third person, they cannot push your friend out and take their place. The
program can tell them apart, ignores the newcomer, and **tells you** that
somebody else is holding your invite — which is worth knowing.

**Only text crosses.** Anything else is dropped rather than shown. And nothing
that arrives can drive your terminal: text from the network is stripped of the
invisible sequences that would let someone move your cursor, repaint your
screen, or reach your clipboard.

**Silence to strangers.** Packets that do not carry the right key get no reply
whatsoever. Someone scanning for open ports gets exactly what they would get
from a closed one, so scanning tells them nothing. It is counted, though, and
you are told, because it means somebody found an address that should only have
been on one invite.

### What "no account" means, and what it does not

There is nothing to sign up for, no profile, no phone number, no server that
records who spoke to whom. Nobody is keeping a list.

**That is not the same as being invisible**, and the difference is worth being
clear about:

- **The person you talk to sees your IP address.** They must — the packets go
  from your home to theirs. That is what "direct" means.
- **The STUN servers see your IP address.** Asking where your packets come from
  means asking someone.
- **Anyone who sees your invite sees your IP address**, because it is written in
  it.

If you need the other person not to know where you are, this is the wrong tool.
That requires routing everything through a third party, which is the exact thing
this avoids.

---

## 8. Dangers

**Treat the invite like a password.** It is the key to the conversation. Send it
somewhere you trust. Anyone who sees it — in a screenshot, a group chat, over
someone's shoulder — can use it.

**If you are told someone else is using your invite, believe it.** The program
says so when packets arrive carrying your key from somewhere that is not your
friend. It cannot be a mistake: random internet noise never carries your key.
Quit and start a new conversation. That gives you a new key and usually a new
address.

**An invite you shared stays valid until you close the program.** There is no
expiry and no way to revoke one. Closing and reopening is the revocation.

**Nothing is protected after the fact.** If someone records your encrypted
traffic today and gets your invite later, they can read that recording. Serious
tools solve this with keys that are thrown away as you go; this one does not.

**In a room, members can lie about who else is present** — they can announce
somebody who does not exist. What they cannot do is read or forge what two other
people say. A made-up member simply never answers and disappears on its own.

**These programs are not signed.** macOS and Windows will warn you, and they are
right to. Check the checksum (section 1) or build it yourself.

**`vapora serve` opens a port on your router.** It asks your router to accept
connections from the internet on a port and closes it again when you quit. It is
protected by an invite the same way everything else is, but it is the one
command here that changes your router's configuration. If it crashes, that door
may stay open until you restart the router.

**Do not rely on this for anything that matters.** It is young, it has never
been reviewed by anybody who breaks software for a living, and the construction
being careful is not the same thing as it being audited.

---

## Layout, for the curious

```
cmd/vapora      the commands: nat, diag, punch, room, probe, serve, connect
pkg/punch       hole punching, rooms, identity, the encrypted packet format
pkg/stun        finding your own address and measuring your router
pkg/pcp         two more ways to ask a router to open a door
pkg/upnp        the most common way to ask a router to open a door
pkg/diag        the experiment that works out which router is the problem
pkg/text        strips anything from the network that could drive a terminal
internal/tui    the full-screen chat
internal/chat   a simpler chat used to prove the UPnP path carries traffic
```

Everything under `pkg/` can be used as a library. `internal/` is scaffolding and
carries no promises.

```bash
go test ./... -race
go vet ./...
```
