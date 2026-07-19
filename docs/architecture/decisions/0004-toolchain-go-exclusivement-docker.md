# ADR-0004 — Toolchain Go exclusivement sous Docker, deux images sandbox

## Statut

Accepté

## Contexte

Le projet vise la reproductibilité et l'absence de prérequis sur l'hôte. Par
ailleurs, un banc dont les `checks` lancent `go test`/`vet`/`build` a besoin d'un
toolchain Go **dans le sandbox**, alors qu'un banc générique n'en a pas besoin.

## Décision

- Toute la chaîne d'outils Go (build, tests, lint, formatters non-Go) tourne
  **exclusivement dans Docker** via `compose.tools.yaml` (images épinglées) et le
  `Makefile` — aucun runtime Go ni `golangci-lint` sur l'hôte. `make build`
  produit tout de même un binaire `./benchy` natif à la plateforme hôte.
- Deux images de bac à sable : `claude-benchy:latest`
  (`build/docker/Dockerfile`, base `node:22-slim`) pour le cas général, et
  `claude-benchy-go:latest` (`build/docker/Dockerfile.golang`, base
  `golang:1.25-bookworm`) embarquant le toolchain Go, ciblée par un banc via
  `sandbox.image`.

## Conséquences

- Environnement identique pour tous les contributeurs, sans installation locale.
- Un banc qui pointe une image absente échoue avec `docker run: exit status 125` ;
  le conteneur du dashboard ne rebâtit pas les images sandbox (les construire au
  préalable : `make image`, `make image-go`).
- La version du CLI `claude` est épinglée dans les Dockerfiles (`CLAUDE_VERSION`),
  surchargeable via `benchy build-image --claude-version`.
