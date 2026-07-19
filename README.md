# claude-benchy

Banc d'essai pour **comparer plusieurs configurations Claude Code**
(skills, rules, `CLAUDE.md`) de façon isolée et reproductible.

On fournit **un prompt** et **une application de test** ; pour chaque config,
`benchy` prépare une copie jetable de l'app, y injecte la config, lance Claude
en headless dans un conteneur Docker, puis compare les résultats :

- le **diff** appliqué au code (`diff.patch`) ;
- le **transcript** complet (`transcript.jsonl`) ;
- les **métriques** (`result.json` : coût, tokens, tours, durée) ;
- un **rapport agrégé** (`report.md` + `report.html`).

Le rapport HTML réunit, en plus du tableau de métriques :

- une **évaluation de l'attendu** (si le banc la demande) : quelle config
  répond le mieux à la tâche, jugée par un LLM selon un *rubric* et étayée par
  des **checks déterministes** (voir `evaluate` plus bas). L'évaluation est
  **agrégée par config** : chaque run exploitable est noté, puis la config
  reçoit le **score moyen** de ses runs et sa **dispersion** (min–max), qui
  mesure la divergence entre runs. Le coût passe au second plan ;
- un **bilan d'efficacité** : points forts / faibles de chaque config (coût,
  rapidité, directivité, périmètre traité) déduits des mesures ;
- une **comparaison côte à côte** : on choisit deux configs et on lit, fichier
  par fichier, le code complet produit de part et d'autre, lignes divergentes
  surlignées.

Le rapport HTML suit le design system **Gazoline** de Motoblouz (noir/blanc
purs, accent jaune `#f1ab00`, Montserrat) et charge Montserrat/Inter depuis
Google Fonts, avec repli sur les polices système hors-ligne. Pour éviter tout
appel externe (RGPD, consultation hors-ligne), les polices peuvent être
auto-hébergées / inlinées en `@font-face` — à demander si besoin.

Cas d'usage : écrire une skill/rule, la décliner en variantes, et voir
laquelle produit la meilleure modification sur une app de référence.

## Prérequis

- **Docker uniquement** : tout le toolchain Go (build, tests, lint) tourne en
  conteneur, rien n'est requis sur l'hôte (cf. Développement). `make build`
  produit tout de même un binaire `./benchy` natif à la plateforme hôte.
- Une session Claude Code authentifiée sur l'hôte : les creds OAuth sont montés
  en lecture seule dans le conteneur. Si le token est expiré, rafraîchis-le sur
  l'hôte (`claude` en interactif) avant de lancer un banc.
- **macOS** : Claude Code range ses creds dans le **Trousseau**, pas dans
  `~/.claude/.credentials.json`. benchy les en exporte automatiquement :
  - le binaire natif (`benchy run`/`serve`) lit le Trousseau à chaque run ;
  - pour le dashboard conteneurisé, `make creds` matérialise l'export dans
    `~/.claude/.credentials.json` (cf. Orbit). Ailleurs (Linux) le fichier
    existant est utilisé tel quel.

## Installation

```sh
make build          # produit ./benchy
make image          # construit l'image de bac à sable claude-benchy:latest
make image-go       # variante avec toolchain Go : claude-benchy-go:latest
```

`make image-go` bâtit l'image sandbox embarquant le toolchain Go, requise par
les bancs dont les `checks` lancent `go test`/`vet`/`build` (p. ex.
`benches/freedy`, qui déclare `sandbox.image: claude-benchy-go:latest`). Un banc
qui pointe une image absente échoue avec `docker run: exit status 125`.

L'image épingle la version du CLI `claude` (défaut : celle du dépôt).
Pour en cibler une autre :

```sh
./benchy build-image --claude-version 2.1.210
```

## Utilisation

```sh
./benchy run examples/bench.yaml
```

Chaque exécution écrit ses artefacts sous `<output>/<horodatage>/`, affiche une
**progression live** (début/fin de chaque config : durée, coût, diff) et le
chemin du rapport HTML en fin de run.

Pour partir d'un banc existant, `benchy new` l'importe en réécrivant ses chemins
relatifs (`app`, `bundle`, `output`…) en absolu par rapport au fichier source,
de sorte que le banc obtenu est lançable depuis n'importe où :

```sh
./benchy new --from benches/freedy/bench.yaml            # écrit sur la sortie standard
./benchy new --from benches/freedy/bench.yaml --output bench.yaml
```

### Interface web

Pour configurer, lancer et parcourir les bancs depuis le navigateur plutôt
qu'en éditant un fichier de banc :

```sh
./benchy serve                 # http://127.0.0.1 (port 80)
```

- **Nouveau bench** : un formulaire couvre tous les inputs du fichier de banc
  (prompt, app, model, runs, configs, rubric, checks…) ; « Lancer » démarre le
  run et **diffuse la progression live**, puis affiche le lien du rapport.
- **Importer une config** : en tête du formulaire, un champ « Importer une
  config depuis un bench.yaml » (avec sélecteur de fichier côté serveur)
  pré-remplit tout le formulaire depuis un `bench.yaml` existant ; ses chemins
  relatifs sont réécrits en absolu par rapport au fichier importé.
- **Sortie live par agent** : sous la console d'orchestration, un panneau par
  agent (config × run) affiche en temps réel la sortie de son Claude — texte,
  appels d'outils (`🔧 Bash`, `✏️ Edit`…) et leurs résultats — rendue à la volée
  depuis le flux stream-json. Chaque panneau porte un lien « transcript ↗ » qui
  rejoue ce même feed depuis `transcript.jsonl` (route `/transcript`) : il reste
  consultable après le run et depuis le rapport (colonne « Détail »), même après
  un redémarrage du serveur.
- **Historique** : la liste de tous les bancs générés sous la racine, chacun
  ouvrant son rapport HTML re-rendu à la volée.
- **Reprendre une config** : depuis un rapport, « Reprendre cette config »
  rouvre le formulaire pré-rempli avec les inputs du banc — pour relire, adapter
  et relancer. Exact quand le run porte son `bench.yaml` (runs lancés depuis
  l'interface **et** via `benchy run`, qui le persiste désormais) ; pour un run
  plus ancien, la config est retrouvée depuis le `bench.yaml` source situé
  au-dessus du dossier de résultats, à défaut reconstruite au mieux.
- **Consulter les fichiers** : les champs app et bundle offrent un bouton 👁 qui
  ouvre une arborescence en lecture seule avec aperçu du contenu de chaque
  fichier — pratique pour vérifier ce que contient un bundle de config ou l'app
  de test avant de lancer.
- **Sélection des chemins** : chaque champ de chemin (app, bundle, dossier de
  sortie, fichier de prompt, configDir) offre un bouton 📁 qui ouvre un
  explorateur de dossiers **côté serveur** — le navigateur ne divulgue pas les
  chemins absolus, c'est donc benchy qui liste le système de fichiers de l'hôte.
  Le glisser-déposer depuis un explorateur de fichiers est géré en meilleur
  effort (il ne fonctionne que lorsque le glisser fournit une URI `file://` —
  fréquent sous Linux/Firefox, aléatoire sous Chrome).

Chaque run lancé depuis l'interface persiste aussi le `bench.yaml` soumis dans
son dossier de résultats (reproductibilité ; rejouable via `benchy run`).

Options : `--addr` (défaut `127.0.0.1:80`), `--root` (dossier scanné pour
l'historique et base des chemins relatifs du formulaire, défaut `.`), `--image`
(image de bac à sable par défaut).

> **Localhost par défaut.** Le serveur peut lancer des conteneurs Docker en
> montant vos creds OAuth, et son explorateur de dossiers liste le système de
> fichiers de l'hôte (lecture seule) : ne l'exposez pas au réseau. `--addr` ne
> s'ouvre à l'extérieur qu'en connaissance de cause.

### Re-générer un rapport sans relancer

Le rapport (`report.md` / `report.html`) se reconstruit à partir des artefacts
déjà présents — pratique après une évolution de la mise en page, sans re-payer
d'appel API :

```sh
./benchy report <output>/<horodatage>
```

### Fichier de banc

```yaml
prompt: |                      # ou promptFile: ./prompts/x.md
  Ajoute un endpoint /health renvoyant 200.
app: ./app-under-test          # dossier de l'app de test (git non requis)
model: sonnet                  # défaut, surchargeable par config
runs: 1                        # répétitions par config (variance)
concurrency: 1                 # runs Docker en parallèle
retries: 2                     # relances d'un run « sans effet » (voir plus bas) ; 0 pour désactiver
auth:
  configDir: ~/.claude         # source des creds OAuth (défaut $CLAUDE_CONFIG_DIR|~/.claude)
sandbox:
  image: claude-benchy:latest  # image du bac à sable ; pointe une image embarquant
                               # le toolchain du projet pour que Claude lance ses checks
output: ./results
keepBaseConfig: true           # garde le CLAUDE.md de l'app comme base et y ajoute
                               # celui de chaque bundle (au lieu de le remplacer)
evaluate:                      # optionnel : évaluer l'adéquation à l'attendu
  model: sonnet                # modèle juge (défaut : model du banc)
  rubric:                      # critères qualitatifs jugés par le LLM
    - "Expose GET /health renvoyant 200"
    - "Modification minimale"
  checks:                      # vérifications déterministes dans le sandbox
    - name: "endpoint déclaré"
      run: "grep -q '/health' server.js"   # passe si la commande sort en 0
    - name: "fichier de test attendu"
      file: "server.test.js"                # passe si le fichier existe
configs:
  - name: baseline
    bundle: ./configs/baseline
  - name: strict-rules
    bundle: ./configs/strict-rules
    model: opus                # override optionnel
    # prompt: ...              # override optionnel
```

Les chemins relatifs sont résolus par rapport au fichier de banc.

### Évaluation de l'attendu

Le bloc `evaluate` (facultatif) déclenche, **en fin de run**, une évaluation de
la qualité des productions :

- les **`checks`** sont des vérifications déterministes rejouées dans le bac à
  sable contre le workspace de chaque config (une commande qui doit sortir en
  `0`, ou un fichier qui doit exister) ; l'image du sandbox doit donc embarquer
  le toolchain nécessaire (cf. `sandbox.image`) ;
- le **`rubric`** liste les critères qualitatifs qu'un **LLM juge** applique :
  il lit les diffs et résumés des runs **exploitables** et note chacun, centré
  sur la tâche.

Les notes par run sont ensuite **agrégées par config** : le score affiché est
la **moyenne** des runs de la config, accompagné de la **dispersion** (min–max)
et d'un niveau **consensuel** par critère. Comparer se fait donc config contre
config, la variance restant lisible run par run dans le tableau de comparaison.

L'évaluation consomme un appel Claude supplémentaire (le juge) ; ses diffs sont
tronqués pour borner le coût. Le résultat est persisté (`evaluation.json`,
`checks.json`) et réaffiché tel quel par `benchy report` — sans nouvel appel.

### Runs sans effet & relances

Un run peut « réussir » côté CLI sans rien produire : aucun appel d'outil, ou un
diff vide — par exemple lorsque le modèle écrit un appel de sous-agent en texte
au lieu de l'exécuter, et termine la session. Un tel run est un **faux succès**
qui fausserait les moyennes (un 0/100 parasite) et les classements d'efficacité
(un run à ~0 s / ~0 $).

`benchy` détecte ces runs **sans effet** et les **relance** jusqu'à `retries`
fois (défaut : 2) pour absorber les ratés transitoires. S'il reste sans effet,
le run est marqué comme tel (statut « sans effet ») et **écarté** de toutes les
agrégations (score, efficacité, juge) tout en restant visible dans le tableau de
comparaison et le détail. Mettre `retries: 0` désactive la relance.

### Bundle de config

Un `bundle` est un dossier superposé tel quel sur la copie de l'app. Y placer
ce qu'on veut tester au niveau projet :

```
configs/strict-rules/
├── CLAUDE.md                  # instructions projet
└── .claude/
    ├── rules/style.md         # règles référencées
    └── skills/<nom>/SKILL.md  # skills projet à évaluer
```

Par défaut, un fichier du bundle **remplace** celui de l'app à même chemin :
un `CLAUDE.md` de bundle écrase donc celui de l'app. Avec
`keepBaseConfig: true`, le `CLAUDE.md` de l'app est **conservé comme base** et
celui du bundle lui est **ajouté à la suite** — on compare alors « base du
projet + surcouche » plutôt que « surcouche seule ». Les fichiers à chemins
distincts (`.claude/rules/*`, `.claude/skills/*`) s'ajoutent dans tous les cas.
Dans l'interface web, l'option est la case « Conserver la config de base de
l'app », et le bouton 👁 permet d'inspecter le contenu de l'app et des bundles.

### Arborescence des résultats

```
results/<horodatage>/
├── <config>/
│   ├── workspace/             # copie isolée après passage de Claude
│   ├── diff.patch             # modifications apportées à l'app
│   ├── transcript.jsonl       # flux stream-json complet
│   ├── result.json            # métriques (coût, tokens, tours, durée)
│   ├── checks.json            # résultats des checks déterministes (si evaluate)
│   └── stdout.log             # sortie d'erreur du conteneur
├── bench.json                 # prompt + app du banc (indexation historique)
├── bench.yaml                 # config du banc (persistée par run CLI et interface web)
├── evaluation.json            # verdict du juge LLM (si evaluate)
├── judge-transcript.jsonl     # transcript du juge (si evaluate)
├── report.md
└── report.html
```

## Orbit (dashboard exposé par hostname)

Le dépôt porte un **module de connexion Orbit** (`.orbit/manifest.yaml`) pour
piloter le dashboard `benchy serve` depuis [Orbit](https://github.com/keyxmare/orbit) :
démarrer / arrêter / suivre les logs en un clic et l'ouvrir sur
**`http://benchy.localhost`** (via le Traefik partagé), sans retenir de port.

Le dashboard est alors conteneurisé (`compose.yaml` + `Dockerfile`). Comme il
lance ses conteneurs sandbox via le daemon Docker de **l'hôte** (docker-out-of-
docker), qui ne sait monter que des chemins hôte, les chemins sont **alignés à
l'identique** dans le conteneur benchy : `compose.yaml` monte la racine des
projets et le dossier des creds Claude au **même chemin absolu**, plus le socket
Docker. Le picker de fichiers et le montage des workspaces/creds dans les
sandbox fonctionnent donc à l'identique de l'exécution sur l'hôte.

### Mise en route

```sh
cp .env.dist .env   # ajuste les chemins hôte si besoin (home, racine projets)
make creds          # macOS : exporte les creds du Trousseau (host, une fois)
make image          # l'image sandbox doit exister sur le daemon hôte
make image-go       # + variante Go si un banc la cible (p. ex. freedy)
```

- `.env` (non versionné) fixe `BENCHY_PROJECTS_ROOT`, `BENCHY_CLAUDE_DIR` et
  `BENCHY_ROOT` — les chemins hôte montés à l'identique. Les défauts visent
  `/Users/keyxmare`.
- **`make creds` se lance sur l'hôte macOS**, pas via Orbit : son runner de
  tâches est un conteneur Linux, sans accès au Trousseau. À relancer quand le
  token a été rafraîchi sur l'hôte.
- **Sous Orbit** : ouvre le projet, lance la tâche « Construit et démarre le
  dashboard » (ou `make up`), puis « Ouvrir » → `benchy.localhost`. Orbit câble
  Traefik et neutralise le port publié ; la santé remonte du healthcheck du
  conteneur.
- **En autonome** (sans Orbit) : `make up` publie aussi le dashboard sur
  `http://127.0.0.1` (port 80). `make down` l'arrête, `make logs` suit ses logs.

> Prérequis : l'image sandbox ciblée par le banc doit être présente sur le
> daemon hôte — `claude-benchy:latest` (`make image`), plus toute variante
> déclarée en `sandbox.image` comme `claude-benchy-go:latest` (`make image-go`)
> — et une session Claude authentifiée sur l'hôte (cf. Prérequis). Le conteneur
> benchy ne rebâtit pas l'image sandbox ; une image absente donne
> `docker run: exit status 125`.

## Isolation & sécurité

- Un conteneur `--rm` jetable par run, exécuté en utilisateur non-root.
- Claude tourne avec `--dangerously-skip-permissions` : le conteneur est le
  bac à sable ; le seul accès disque est la copie montée de l'app. Le réseau
  reste ouvert (l'API Claude en a besoin).
- Isolation des réglages via `--setting-sources project,local` : les
  settings/skills utilisateur de l'hôte ne contaminent pas le test. Seuls les
  creds OAuth sont montés.

## Développement

Le toolchain Go tourne **exclusivement dans Docker** (`compose.tools.yaml`) —
aucun runtime Go, ni `golangci-lint`, sur l'hôte.

```sh
make check          # format (gofmt), vet, lint (golangci-lint), tests
make check-fast     # idem sans les tests (contrat du hook commit-gate)
make test           # go test ./...
make fmt            # gofmt -w .
make build          # binaire ./benchy natif à la plateforme hôte
```

Docker est aussi utilisé au runtime par l'outil pour les bacs à sable Claude.
