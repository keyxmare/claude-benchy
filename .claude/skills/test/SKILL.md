---
name: test
description: Écrit/complète puis lance les tests Go ciblés couvrant le changement en cours, exclusivement via Docker. Red-green obligatoire pour un fix. À invoquer pour "teste", "couvre", "ajoute des tests", "test de non-régression", "/test". Lancer la suite sans rien écrire relève de /check.
argument-hint: [périmètre — fichier, feature ou fix concerné]
allowed-tools: Read, Grep, Glob, Edit, Write, Bash(make:*), Bash(docker compose:*), Bash(git:*)
---

# test — couvrir le changement en cours

La **démarche** détaillée (idiomes, table-driven, golden files) relève de la
skill `go-testing` ; les **conventions** de `.claude/rules/go-tests.md`
(couverture 100 %, boîte noire, `go-cmp`, déterminisme) sont obligatoires.

## 1. Périmètre & niveau

Identifier ce qui a changé (commits de la branche + working tree) et choisir
le niveau : unitaire (logique pure, fonctions pures comme la construction de
la commande `docker run`), fonctionnel (parsing de flux, capture de diff sur
fixtures). S'aligner sur les tests existants les plus proches (structure,
nommage, helpers).

Suite déjà rouge avant le changement, hors périmètre → le signaler, ne pas
corriger ces échecs dans ce run.

## 2. Fix → red-green

Pour une correction de bug : écrire d'abord le test de non-régression qui
reproduit le bug, vérifier qu'il échoue, appliquer le correctif, vérifier
qu'il passe. Jamais dans l'autre ordre.

## 3. Écriture

- Boîte noire par défaut (`package xxx_test`), AAA, table-driven dès que
  plusieurs cas partagent la logique, sous-tests `t.Run` nommés.
- Déterministes : pas de Docker/réseau/API réels, pas d'horloge murale ni
  d'ordre. Piloter les ressources externes (`docker.Runner`, horloge)
  derrière un fake ; tester les **fonctions pures** et les **branches de
  garde** (entrée invalide, run sans effet, retours anticipés).
- `github.com/google/go-cmp/cmp` pour les structures ; messages *got avant
  want*. Sorties volumineuses (rapports) → golden files sous `testdata/` (flag
  `-update`).
- Couverture 100 % du code visé, cas limites compris (bornes, vides, nil,
  erreurs).

## 4. Exécution — Docker uniquement

Ciblé d'abord, via le conteneur `go` du toolchain :

- un package : `docker compose -f compose.tools.yaml run --rm go go test ./internal/spec/ -v`
- un test : `… go test ./internal/spec/ -run TestLoad -v`
- course : `… go test -race ./...`

Élargir ensuite ; le wrapper complet `make test` en dernier. La suite complète
des gates relève de `/check`.

## 5. Sortie

Tests ajoutés/modifiés, résultat de la suite ciblée (verte). Couverture :
`docker compose -f compose.tools.yaml run --rm go go test ./... -cover` (au
besoin `-coverprofile` puis `go tool cover -func`) pour vérifier 100 % sur le
code visé ; sinon lister les chemins non couverts et les combler.
