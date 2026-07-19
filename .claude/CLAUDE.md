# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Présentation

`benchy` est un banc d'essai qui **compare plusieurs configurations Claude Code**
(skills, rules, `CLAUDE.md`) sur un même prompt et une même app de test, de façon
isolée et reproductible. Pour chaque config, il prépare une copie jetable de
l'app, y superpose la config, lance Claude en headless dans un conteneur Docker,
puis capture diff, transcript, métriques et produit un rapport agrégé. Le README
détaille le fonctionnement, le format du fichier de banc et l'usage.

## Commandes

Le toolchain Go tourne **exclusivement dans Docker** (`compose.tools.yaml`) —
aucun runtime Go ni `golangci-lint` sur l'hôte. Passer par le Makefile :

```sh
make check          # gate complet : fmt-check → vet → lint → test
make check-fast     # idem sans les tests (contrat du hook commit-gate)
make test           # go test ./...
make fmt            # gofmt -w .
make build          # binaire ./benchy natif à la plateforme hôte
make image          # image sandbox claude-benchy:latest
make image-go       # variante embarquant le toolchain Go (bancs à checks go test)
```

Lancer un seul test (le toolchain Go passe par `make`/Docker, mais on peut aussi
invoquer directement le conteneur `go`) :

```sh
docker compose -f compose.tools.yaml run --rm go go test ./internal/spec/ -run TestLoad -v
docker compose -f compose.tools.yaml run --rm go go test ./... -cover   # couverture
```

Golden files : les rapports (`internal/report/testdata/*.golden`) se régénèrent
avec le flag `-update` sur le test concerné.

## Architecture

Le flux `run` est un pipeline linéaire orchestré par `internal/runner`, réutilisé
tel quel par la CLI (`cmd/benchy`) et le dashboard web (`internal/server`) — les
deux appellent `runner.Run`, `runner.Reload` et `report.Write`. Comprendre le
projet, c'est suivre le chemin d'un banc à travers ces packages :

- **`internal/spec`** — charge et valide le fichier de banc YAML (`Spec`,
  `Config`, `evaluate`), résout les chemins relatifs au fichier de banc, et
  localise les creds OAuth (`creds.go`, avec l'export Trousseau sur macOS).
- **`internal/workspace`** — pour chaque config, copie l'app dans un dossier
  isolé (seulement l'arbre `HEAD` si l'app est un dépôt git), superpose le bundle
  de config, et **commit une baseline git**. La baseline inclut le bundle pour
  qu'un diff ultérieur ne surface que les changements de Claude. `keepBaseConfig`
  décide si le `CLAUDE.md` du bundle **remplace** ou **s'ajoute** à celui de l'app.
- **`internal/docker`** — construit et lance le conteneur sandbox `--rm` en
  non-root. La construction de la commande `docker run` est une **fonction pure**
  (testable sans daemon). Claude y tourne avec `--dangerously-skip-permissions`
  et `--setting-sources project,local` (les settings/skills hôte ne contaminent
  pas le test) ; seuls les creds OAuth sont montés.
- **`internal/claude`** — parse le flux `stream-json` du transcript
  (`stream.go` : métriques finales, coût, tokens, tours) et le **rend en lignes
  lisibles** (`render.go` : texte, appels d'outils, résultats) pour la sortie
  live et la page `/transcript`.
- **`internal/diffcap`** — capture les changements du workspace en diff unifié
  contre la baseline git (`diff.patch` + stats).
- **`internal/runner`** — le cœur : `runner.go` ordonnance les jobs
  (config × run, `concurrency` en parallèle), détecte les **runs sans effet**
  (aucun outil / diff vide) et les **relance** jusqu'à `retries` fois ;
  `judge.go` fait tourner le **juge LLM** de `evaluate` (rubric + checks
  déterministes rejoués dans le sandbox) ; `reload.go` reconstruit le rapport
  depuis les artefacts sans re-payer d'appel API.
- **`internal/report`** — agrège les résultats par run en `report.md` +
  `report.html` (design system **Gazoline** de Motoblouz), avec agrégation par
  config (score moyen + dispersion) et comparaison côte à côte fichier par
  fichier.
- **`internal/server`** — dashboard web (bibliothèque standard uniquement) :
  formulaire de banc, progression live (SSE), sortie par agent, historique,
  reprise de config, pickers de fichiers côté serveur. **Localhost par défaut**
  (il lance des conteneurs avec creds montés et liste le FS hôte).

Point clé : un run peut « réussir » côté CLI sans rien produire (faux succès) ;
la détection des runs **sans effet** et leur relance/écartement des agrégations
est une invariante métier centrale, à préserver lors des modifications du runner.

## Conventions de test

Les tests Go de ce projet suivent une **exigence de couverture à 100 %**
(instructions **et** branches, chemins d'erreur compris). Voir
`.claude/rules/go-tests.md` pour les conventions détaillées (toujours actives) et
la skill `go-testing` pour la démarche. En résumé : boîte noire par défaut
(`package xxx_test`), table-driven, AAA, `go-cmp` (pas de testify ni
`reflect.DeepEqual`), messages *got avant want*, golden files pour les grosses
sorties, et `go test -race` doit passer.

## Git

Commit direct sur `main` autorisé ; branche + PR au jugé pour les sujets
conséquents ou risqués. Forge GitHub, CI GitHub Actions.
