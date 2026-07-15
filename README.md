# claude-benchy

Banc d'essai pour **comparer plusieurs configurations Claude Code**
(skills, rules, `CLAUDE.md`) de façon isolée et reproductible.

On fournit **un prompt** et **une application de test** ; pour chaque config,
`benchy` prépare une copie jetable de l'app, y injecte la config, lance Claude
en headless dans un conteneur Docker, puis compare les résultats :

- le **diff** appliqué au code (`diff.patch`) ;
- le **transcript** complet (`transcript.jsonl`) ;
- les **métriques** (`result.json` : coût, tokens, tours, durée) ;
- un **rapport agrégé** côte à côte (`report.md` + `report.html`).

Cas d'usage : écrire une skill/rule, la décliner en variantes, et voir
laquelle produit la meilleure modification sur une app de référence.

## Prérequis

- Go (build de l'outil) et Docker (exécution des bacs à sable).
- Une session Claude Code authentifiée sur l'hôte : les creds OAuth
  (`~/.claude/.credentials.json` par défaut) sont montés en lecture seule dans
  le conteneur. Si le token est expiré, rafraîchis-le sur l'hôte
  (`claude` en interactif) avant de lancer un banc.

## Installation

```sh
make build          # produit ./benchy
make image          # construit l'image de bac à sable claude-benchy:latest
```

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
auth:
  configDir: ~/.claude         # source des creds OAuth (défaut $CLAUDE_CONFIG_DIR|~/.claude)
sandbox:
  image: claude-benchy:latest  # image du bac à sable ; pointe une image embarquant
                               # le toolchain du projet pour que Claude lance ses checks
output: ./results
configs:
  - name: baseline
    bundle: ./configs/baseline
  - name: strict-rules
    bundle: ./configs/strict-rules
    model: opus                # override optionnel
    # prompt: ...              # override optionnel
```

Les chemins relatifs sont résolus par rapport au fichier de banc.

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

### Arborescence des résultats

```
results/<horodatage>/
├── <config>/
│   ├── workspace/             # copie isolée après passage de Claude
│   ├── diff.patch             # modifications apportées à l'app
│   ├── transcript.jsonl       # flux stream-json complet
│   ├── result.json            # métriques (coût, tokens, tours, durée)
│   └── stdout.log             # sortie d'erreur du conteneur
├── report.md
└── report.html
```

## Isolation & sécurité

- Un conteneur `--rm` jetable par run, exécuté en utilisateur non-root.
- Claude tourne avec `--dangerously-skip-permissions` : le conteneur est le
  bac à sable ; le seul accès disque est la copie montée de l'app. Le réseau
  reste ouvert (l'API Claude en a besoin).
- Isolation des réglages via `--setting-sources project,local` : les
  settings/skills utilisateur de l'hôte ne contaminent pas le test. Seuls les
  creds OAuth sont montés.

## Développement

```sh
make check          # gofmt -l, go vet, golangci-lint (si présent), go test
```

Le toolchain de dev (Go) tourne sur l'hôte ; Docker n'est utilisé qu'au
runtime par l'outil pour les bacs à sable Claude.
