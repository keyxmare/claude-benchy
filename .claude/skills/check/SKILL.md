---
name: check
description: Lance les gates qualité du projet — format → lint → analyse statique → tests — exclusivement via Docker. Contrat make check/check-fast d'abord. À invoquer sur demande — "lance les checks", "vérifie que ça passe", "lint", "c'est vert ?", "/check".
allowed-tools: Read, Grep, Glob, Bash(make:*), Bash(docker compose:*)
---

# check — gates qualité (Docker)

Vérifie le code dans l'ordre des gates : **format → lint → analyse statique →
tests**. Le formatteur peut modifier des fichiers → les signaler. La famille
tests inclut le gate de couverture (100 % sur le code visé, cf.
`.claude/rules/go-tests.md`).

## 1. Le contrat d'abord

Le point d'entrée est le `Makefile`, tout le toolchain Go passe par Docker
(`compose.tools.yaml`) :

- `make check` par défaut (suite complète : Go `fmt-check → vet → lint` +
  shellcheck, shfmt, hadolint, yamllint, markdownlint → test).
- `make check-fast` uniquement sur demande explicite (contrat du hook
  `commit-gate`, sans les tests).

Lire la recette, confirmer qu'elle passe par Docker, l'exécuter. Chemin
nominal, terminé.

## 2. Gates par famille (si diagnostic ciblé)

Pour isoler une famille sans relancer toute la suite (toujours via Docker) :

| Famille | Cible |
| --- | --- |
| format | `make fmt-check` (correction : `make fmt`) |
| lint | `make lint` (golangci-lint) + `make shellcheck shfmt hadolint yamllint markdownlint` |
| analyse statique | `make vet` |
| tests + couverture | `make test` ; `docker compose -f compose.tools.yaml run --rm go go test ./... -cover` |

Recette qui appellerait un binaire Go hôte (hors conteneur) → la signaler
comme non conforme, ne pas l'exécuter. Ne pas corriger le Makefile dans ce
run.

## 3. Règles d'exécution

- S'arrêter à la première famille en échec ; ne pas relancer une famille
  déjà verte si rien n'a bougé.
- Pas de wrapper Docker pour un outil → stop et demander.

## 4. Sortie

Statut par famille (✓ / ✗ / sans objet), fichiers modifiés par le
formatteur, extrait utile de l'erreur en cas de ✗. La correction relève de
l'appelant.
