```
██      ██      ██      ████████      ██████    ████████        ██
██      ██    ██  ██    ██      ██  ██      ██  ██      ██    ██  ██
██      ██  ██      ██  ██      ██  ██      ██  ██      ██  ██      ██
██      ██  ██      ██  ████████    ██      ██  ████████    ██      ██
██      ██  ██████████  ██          ██      ██  ██    ██    ██████████
  ██  ██    ██      ██  ██          ██      ██  ██      ██  ██      ██
    ██      ██      ██  ██            ██████    ██      ██  ██      ██
```

[English](../../README.md) · [Español](../es/README.md) · [中文](../zh/README.md) · [日本語](../ja/README.md) · [Português](../pt/README.md) · [العربية](../ar/README.md) · **Français** · [Italiano](../it/README.md) · [Deutsch](../de/README.md) · [Русский](../ru/README.md)

### Discutez directement d'un ordinateur à l'autre. Sans serveur. Sans compte. Sans trace.

Vous partagez une ligne de texte. L'autre la colle. Vous discutez déjà —
chiffré, en direct, sans rien au milieu.

[![release](https://img.shields.io/github/v/release/MalPr0/vapora?style=flat-square&color=e8a33d)](https://github.com/MalPr0/vapora/releases/latest)
![go](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square)
![dépendances](https://img.shields.io/badge/dépendances-zéro-2ea043?style=flat-square)
![licence](https://img.shields.io/badge/licence-MIT-blue?style=flat-square)

---

## Essayez en 30 secondes

```bash
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz
./vapora punch
```

Il affiche une ligne. Envoyez-la à quelqu'un. Cette personne la colle dans son
terminal.

<sup>Autres versions : `darwin_amd64` · `linux_amd64` · `linux_arm64` · `windows_amd64.zip` — remplacez le nom dans l'URL. Utilisez `curl`, pas votre navigateur : un navigateur marque ce qu'il télécharge comme non fiable et macOS refuse ensuite de l'exécuter.</sup>

---

## À quoi ça ressemble

```
 █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄                    ● JADE HERON     31ms
 █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █                   ● SWIFT OTTER    47ms
 ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█                   ◐ GREY MARTEN  no reply 9s
   ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ you are CRIMSON QUAIL ━━━━━━━━━━━━━━━━━━━━━━━━━

  --             JADE HERON joined
  JADE HERON     il y a quelqu'un ?
  SWIFT OTTER    @QUAIL regarde ça
▸ CRIMSON QUAIL  j'arrive
  GREY MARTEN    ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
> hola_
                        enter sends · pgup/pgdn scrolls · !exit quits
```

Un chat de terminal en pixel art rétro. Chacun reçoit un nom d'animal que
personne ne peut revendiquer, les `@mentions` extraient une ligne du défilement,
et un petit coureur traverse l'écran de chargement pendant que la connexion se
fraye un chemin.

---

## Pourquoi ça peut vous servir

**Personne n'est au milieu.** Vos mots vont de votre machine à la sienne. Ni par
les serveurs d'une entreprise, ni par les miens. Il n'y a pas de milieu à
assigner en justice, à revendre ou à pirater.

**Rien à créer comme compte.** Pas d'e-mail, pas de numéro, pas d'identifiant,
pas de profil. Le programme ne sait pas qui vous êtes, et personne d'autre non
plus.

**Rien n'est conservé.** Fermez-le et la conversation disparaît des deux côtés.
Il n'y a pas d'historique à fuiter, parce qu'il n'y a pas d'historique.

**Un fichier, zéro dépendance.** Téléchargez un binaire et lancez-le. Pas de
Docker, pas de runtime, pas d'installation. Construit avec la bibliothèque
standard de Go et rien d'autre — vous pouvez lire chaque ligne distribuée.

**Chiffré par défaut, sans moyen de le désactiver.** AES-256-GCM, une clé
différente par direction. L'invitation que vous partagez *est* la clé.

**Les groupes sont un vrai maillage.** Chacun parle directement à chacun. Deux
personnes dans un salon de cinq ont un canal que les trois autres ne peuvent pas
lire — non pas comme promesse de comportement, mais par arithmétique : elles
n'ont pas les clés.

---

## À quoi les gens s'en servent

- **Envoyer quelque chose de sensible** à un collègue sans que ça reste pour
  toujours dans le journal de discussion d'une entreprise.
- **Discuter à travers un pare-feu** là où vous ne pouvez ni ouvrir de port ni
  installer quoi que ce soit.
- **Un canal rapide avec quelqu'un**, sans compte, sans historique et sans trace
  sur aucune des deux machines.
- **Comprendre votre propre connexion** — le diagnostic vous en dit plus sur
  votre réseau que votre fournisseur.

---

## À deux

```bash
./vapora punch                     # vous : affiche une invitation
./vapora punch "<l'invitation>"    # l'autre : la colle et l'exécute
```

**Si ça ne connecte pas, envoyez chacun une invitation.** Les box domestiques
refusent généralement les paquets d'inconnus, donc quand les deux font ça, le
premier paquet de chacun meurt à la porte de l'autre. L'écran de l'autre affiche
une ligne sous *"if it does not connect, send this back"* — demandez-la,
collez-la dans votre terminal, et vous frappez maintenant tous les deux au même
moment. C'est exactement ce que ces box ont besoin de voir.

Vous pouvez savoir à l'avance si cette étape sera nécessaire — voir
[diagnostic](#connaissez-votre-réseau-avant-de-laccuser).

## En groupe

```bash
./vapora room                      # ouvre un salon et affiche une invitation
./vapora room "<l'invitation>"     # n'importe qui entre avec elle
```

**N'importe qui peut inviter.** Arrivé il y a cinq minutes ? `!invite` vous donne
une ligne pour faire venir la personne suivante. Tout le monde finit par
connaître tout le monde sans repasser par celui qui a ouvert le salon.

**Celui qui vous a invité n'est pas un serveur.** Il présente deux personnes puis
s'écarte. Il ne transporte rien entre elles et ne pourrait pas le lire même en
essayant. Éteignez la machine qui a ouvert le salon : la conversation continue
sans elle.

**Les salons tiennent huit personnes**, et **se ferment une fois vides** — un
salon où il n'y a personne est un port sans propriétaire. Ajoutez `-standalone`
si vous voulez qu'un salon reste à attendre.

**Vous êtes deux sur le même wifi ?** Ça marche aussi. Chaque participant annonce
à la fois son adresse publique et sa locale, parce que deux machines derrière la
même box ne peuvent pas s'atteindre par la publique. Ça se règle tout seul en
quelques secondes.

### Pendant que vous y êtes

| | |
|---|---|
| `@nom` | extrait votre ligne du défilement de l'autre, avec une marque en marge |
| `!who` | qui est là, et la santé de chaque connexion |
| `!invite` | une nouvelle invitation pour faire venir quelqu'un |
| `!exit` | partir, en prévenant tout le monde immédiatement |
| `PgUp` / `PgDn` | remonter dans ce qui a été dit |
| `-plain` | des lignes simples au lieu du plein écran, quand quelque chose cloche |

---

## Comment ça marche

Votre ordinateur n'a pas d'adresse propre sur internet. C'est votre box qui en a
une, et tout ce qu'il y a chez vous la partage. C'est le **NAT**, et c'est
pourquoi personne ne peut simplement « appeler » votre portable. La réponse
habituelle est de mettre un serveur au milieu auquel les deux côtés se
connectent *vers l'extérieur* — ça marche, et ça veut dire que l'ordinateur de
quelqu'un d'autre voit chaque mot.

vapora fait l'inverse. Les deux côtés envoient des paquets *vers l'extérieur* au
même moment, chacun perçant un trou dans sa propre box, et les deux trous
s'alignent. Ensuite le chemin est direct et personne d'autre n'est dessus.

| Quoi | Pourquoi c'est là |
|---|---|
| **UDP hole punching** | Le chemin direct lui-même. Les deux côtés percent en même temps et se rejoignent au milieu. |
| **STUN** ([5389](https://www.rfc-editor.org/rfc/rfc5389), [5780](https://www.rfc-editor.org/rfc/rfc5780)) | Découvre l'adresse que le monde extérieur voit, et classe le comportement de votre box. |
| **UPnP-IGD, PCP, NAT-PMP** | Trois langues pour demander à une box d'ouvrir une porte. Les trois sont tentées, car les box s'accordent rarement sur celle qu'elles parlent. |
| **X25519 + HKDF + AES-256-GCM** | Une clé distincte par paire et par direction. Dans un salon, aucun membre ne lit le trafic d'une autre paire. |
| **Fenêtre anti-rejeu** | Fenêtre glissante façon IPsec, par émetteur, pour qu'un paquet capturé ne puisse pas vous être rejoué. |
| **DHT BitTorrent** *(optionnel)* | Se trouver sans aucune adresse. Désactivé par défaut — voir [sécurité](#sécurité). |

Tout vient de la bibliothèque standard de Go. Aucun code tiers, nulle part.

<sup><a href="../../ARCHITECTURE.md">ARCHITECTURE.md</a> contient le pas à pas, avec des schémas.</sup>

---

## Connaissez votre réseau avant de l'accuser

```bash
./vapora nat                   # quel type de box vous avez devant vous
./vapora diag                  # chaque routeur entre vous et internet
```

`nat` affiche un profil court du genre `CONE-PORT-18`. Envoyez-le à la personne
avec qui vous voulez vous connecter, entrez le sien, et il vous dit à quoi
vous attendre **avant** de perdre une soirée :

```bash
./vapora nat -pair CONE-OPEN-64                    # à deux
./vapora nat -room "CONE-PORT-18,SYM-PORT-F3"      # pour tout un salon
```

Qu'une connexion fonctionne est une propriété de la *paire*, pas de l'un des deux
bouts — aucune mesure de votre propre réseau n'y répond seule. C'est pour ça que
le profil est fait pour être collé à quelqu'un d'autre. Pour un salon il va plus
loin : il dit si le maillage se referme, qui devrait héberger, et exactement
quelle paire ne s'atteindra jamais.

<sup>Si un pare-feu ouvre un port précis, mesurez celui-là : <code>vapora nat -port 41000</code>. Le filtrage est une propriété d'un port, pas d'une machine.</sup>

---

## Sécurité

**L'invitation est la clé.** Cette chaîne n'est pas une adresse, c'est le secret
qui chiffre tout. Traitez-la comme un mot de passe : quiconque la voit — sur une
capture, dans un groupe, par-dessus votre épaule — peut s'en servir.

**Silence face aux inconnus.** Les paquets sans la bonne clé n'obtiennent aucune
réponse. Un scanner de ports apprend exactement ce qu'il apprendrait d'un port
fermé. Mais ils sont comptés, et **on vous le dit**, parce que ça veut dire que
quelqu'un a trouvé une adresse qui n'aurait dû figurer que sur une invitation.

**Personne ne peut prendre votre conversation.** Donnez votre invitation à une
troisième personne : elle ne pourra toujours pas évincer votre ami. Le programme
les distingue, ignore le nouveau venu, et vous prévient.

**Seul le texte passe.** Tout le reste est jeté plutôt qu'affiché. Et le texte
venu du réseau est débarrassé des séquences d'échappement qui permettraient de
déplacer votre curseur, redessiner votre écran ou atteindre votre presse-papiers.

**Une invitation reste valable jusqu'à ce que vous fermiez le programme.** Elle
n'expire pas et ne peut pas être révoquée. Fermer et rouvrir *est* la révocation
— ça vous donne une nouvelle clé et en général une nouvelle adresse.

**Dans un salon, un membre peut mentir sur qui d'autre est présent.** Il peut
annoncer quelqu'un qui n'existe pas. Ce qu'il ne peut pas faire, c'est lire ou
falsifier ce que deux autres se disent. Un membre inventé ne répond jamais et
décroche tout seul.

**« Sans compte » n'est pas la même chose qu'invisible.** La personne à qui vous
parlez voit votre adresse IP. Forcément — les paquets vont de chez vous à chez
elle. C'est ce que *direct* veut dire, et c'est le prix honnête de l'absence de
serveur.

**`-discover` publie votre adresse sur un réseau public**, et c'est pour ça que
c'est désactivé par défaut. Avec cette option, les deux côtés se trouvent via le
DHT BitTorrent sous un nom dérivé de votre secret. Personne ne peut vous chercher
sans ce secret, mais vous devenez une ligne de plus dans une table que n'importe
qui peut parcourir.

---

## Ce qui va casser, et quand

Des limites honnêtes, pas des petites lignes.

- **Les serveurs STUN appartiennent à d'autres** — Google, Cloudflare et deux
  autres, des services gratuits qui existent pour autre chose. S'ils
  disparaissent, ceci ne peut plus découvrir sa propre adresse, et il n'y a pas
  de solution de repli aujourd'hui.
- **Certains réseaux le bloquent purement et simplement** : entreprises,
  universités, hôtels, certains opérateurs mobiles. Rien de votre côté n'y peut
  quelque chose.
- **Certaines connexions en sont tout simplement incapables.** Un NAT
  *symétrique* ou opérateur rend votre adresse imprévisible d'un instant à
  l'autre : il n'y a rien à viser. `vapora nat` vous le dit. La seule solution
  est un relais, que ceci n'a délibérément pas.
- **Votre adresse change et l'invitation meurt.** Changer de wifi, passer en
  données mobiles, rester inactif assez longtemps. Le programme s'en aperçoit et
  en affiche une nouvelle, mais c'est à vous de la renvoyer.
- **Les versions doivent correspondre.** Le format a déjà changé plusieurs fois
  et changera encore. Ancien et nouveau ne s'entendent pas, et le symptôme est le
  *silence*. Lancez `./vapora version` des deux côtés d'abord.
- **Rien n'est protégé rétroactivement.** Quelqu'un qui enregistre votre trafic
  aujourd'hui et obtient votre invitation plus tard pourra lire cet
  enregistrement. Les outils sérieux règlent ça avec des clés jetées au fur et à
  mesure. Pas celui-ci.
- **Les binaires ne sont pas signés.** Votre système vous avertira, et il a
  raison. Vérifiez la somme de contrôle avec `SHA256SUMS`, ou compilez vous-même.
- **`vapora serve` modifie la configuration de votre box.** C'est la démo UPnP
  d'origine, et la seule commande ici qui demande à votre box d'ouvrir un port
  sur internet. Elle le referme en quittant — mais en cas de plantage, cette
  porte peut rester ouverte jusqu'au redémarrage de la box. Tout le reste de ce
  README ne touche pas à votre box.
- **Personne dont le métier est de casser des logiciels n'a relu ceci.** Être
  construit avec soin n'est pas la même chose qu'être audité. Ne misez rien
  d'important dessus.

---

## Comment vous pouvez vous en servir

<sup><code>ARCHITECTURE</code> et le tutoriel Pong, liés ci-dessous, n'existent pour l'instant qu'en anglais.</sup>

Le chat est une chose construite sur le canal, pas son but. Le transport est une
couche à part qui n'a aucune idée de ce qu'est une conversation : il ouvre un
chemin chiffré à travers deux box, maintient un maillage en vie, et déplace des
**octets**.

Quarante lignes suffisent à un programme qui fonctionne — deux exemplaires, sur
deux machines n'importe où sur internet, s'échangeant des octets sans rien au
milieu :

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
mux := punch.NewMux(conn)
session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)

session.Observe(punch.ObserverFunc(func(payload []byte) {
    fmt.Println("←", string(payload))       // exactement ce qui a été envoyé
}))

go mux.Run(ctx)
go session.Run(ctx)

session.Open(ctx, 3*time.Minute)             // perce les deux box
session.Send([]byte("hola"))
```

### 🏓 Commencez ici : [**construire un Pong**](../../examples/pong/README.md)

Un tutoriel pas à pas qui va de ce squelette à un vrai jeu à deux à travers
internet — son propre format réseau, qui a le droit d'avoir raison sur quoi, et
pourquoi un jeu survit à une perte de paquets qui ruinerait une conversation.

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

### Trois choses sur un même canal

| | Envoie | Se soucie de |
|---|---|---|
| **[Pong](../../examples/pong/README.md)** — tutoriel | de l'**état**, 30 fois par seconde | seulement du plus récent. Un paquet perdu coûte une image |
| **[filedrop](../../examples/filedrop)** | des **blocs** d'un fichier | de tous, et à la bonne place |
| **`vapora punch` / `room`** | des **événements** — des lignes de texte | de chacune d'elles |

Un jeu et une conversation veulent des choses opposées du même transport — la
fraîcheur contre la livraison — et aucun des deux n'a demandé au transport de
changer. C'est la preuve la plus claire que la séparation en couches est réelle,
et c'est pourquoi construire dessus ne signifie pas hériter des décisions de
quelqu'un d'autre.

### Les paquets

| Paquet | Ce qu'il vous donne |
|---|---|
| `pkg/punch` | Le chemin, le chiffrement, le maillage. Des octets entrent, des octets sortent. |
| `pkg/stun` | Votre adresse publique, et une classification de votre NAT. |
| `pkg/upnp`, `pkg/pcp` | Demander à une box d'ouvrir une porte, en trois protocoles. |
| `pkg/dht` | Annoncer et trouver une adresse sur le DHT BitTorrent. |
| `pkg/diag` | Si deux réseaux peuvent s'atteindre, et quoi faire. |
| `pkg/names` | Une clé transformée en un nom qu'une personne peut prononcer. |
| `pkg/chat` | Lignes, saisie et locuteurs — la couche qu'utilise l'interface de ce programme. |

**→ [ARCHITECTURE.md](../../ARCHITECTURE.md) parcourt le tout** : comment un chemin
s'ouvre étape par étape, à quoi ressemble le format réseau, comment le maillage
se génère ses clés, et une recette pour construire dessus. Des schémas, pas de la
prose.

---

## Compilez-le vous-même

La réponse la plus courte à « dois-je faire confiance à ce binaire ? » :

```bash
git clone https://github.com/MalPr0/vapora && cd vapora
go build ./cmd/vapora
go test ./... -race
```

Go 1.25. Rien à télécharger, rien à configurer.

Chaque déclaration exportée de `pkg/` est documentée, et la vérification est dans
le dépôt : `go run ./internal/doclint pkg`.

**L'organisation, si vous voulez lire.** `pkg/punch` c'est la poignée de main,
les sessions et les salons. `pkg/stun` découvre votre adresse et classe votre
NAT. `pkg/upnp` et `pkg/pcp` demandent aux box d'ouvrir des portes. `pkg/dht` est
le client BitTorrent. `pkg/diag` est le raisonnement derrière les conseils.
`internal/tui` est le chat en pixel art.

[`ARCHITECTURE.md`](../../ARCHITECTURE.md) est la visite guidée de tout ça.
[`AGENTS.md`](../../AGENTS.md) documente les invariants — ce qui ressemble à des
détails et se révèle porteur. Celui-là est uniquement en anglais, parce que c'est
la référence de travail du code.

---

<sup>Licence MIT. Construit à découvert, un commit à la fois.</sup>
