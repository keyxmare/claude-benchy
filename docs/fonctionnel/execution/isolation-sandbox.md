---
titre: Isolation du bac à sable
public: tous
sources:
  - internal/docker/docker.go
  - internal/runner/runner.go
  - build/docker/Dockerfile
  - build/docker/Dockerfile.golang
---

# Isolation du bac à sable

> Chaque run tourne dans un conteneur jetable et isolé : Claude y travaille en
> autonomie, sans polluer l'hôte ni être influencé par les réglages personnels de
> l'opérateur.

## User story

En tant qu'utilisateur, je veux que chaque run s'exécute dans un environnement
propre et isolé, afin que le résultat mesure la config du banc et non ma
configuration locale, et sans effet de bord sur ma machine.

## Ce qui est garanti

- Un conteneur `--rm` **jetable** par run, en utilisateur **non-root**.
- Claude tourne avec `--dangerously-skip-permissions` : le conteneur **est** le
  bac à sable ; le seul accès disque est la copie montée de l'app. Le réseau reste
  ouvert (l'API Claude en a besoin).
- Isolation des réglages via `--setting-sources project,local` : les
  settings/skills utilisateur de l'hôte ne contaminent pas le test.
- Seuls les creds OAuth sont montés (en lecture seule).

Voir [ADR-0003](../../architecture/decisions/0003-isolation-sandbox-et-reglages.md)
et la règle [RG-exec-04](../../domaine/execution/index.md#règles).

## Deux images de bac à sable

- `claude-benchy:latest` — cas général (base Node).
- `claude-benchy-go:latest` — embarque le toolchain Go, pour un banc dont les
  checks lancent `go test`/`vet`/`build` (ciblée via `sandbox.image`).

Voir [ADR-0004](../../architecture/decisions/0004-toolchain-go-exclusivement-docker.md).

## Critères d'acceptation

- **Given** un run, **When** la commande Docker est construite, **Then** elle
  contient `--rm`, monte le workspace en `/work` et (si fourni) les creds en
  lecture seule, sans monter `~/.claude` ni les réglages de l'hôte.
- **Given** un run Claude, **When** ses arguments sont construits, **Then** ils
  incluent `--dangerously-skip-permissions` et `--setting-sources project,local`.
- **Given** aucun fichier de creds, **When** la commande est construite, **Then**
  aucun montage de creds n'est ajouté.

## Cas non triviaux & limites

- Le non-root vient du `USER` des Dockerfiles, pas d'un flag `--user`.
- Une image absente fait échouer le run (`docker run: exit status 125`) ; le
  dashboard conteneurisé ne rebâtit pas les images.
- Le réseau ouvert est un compromis assumé (dépendance à l'API Claude).

## Cas de test associés

`internal/docker/docker_test.go` : `TestRunArgs`, `TestRunArgsWithoutCreds`,
`TestRunArgsWithEntrypoint`.
`internal/runner/runner_test.go` : `TestClaudeArgsForceNonInteractive`.
