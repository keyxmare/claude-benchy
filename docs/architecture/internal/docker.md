---
titre: internal/docker — lancement du bac à sable
public: dev
sources:
  - internal/docker/docker.go
---

# internal/docker — lancement du bac à sable

> Lance le conteneur sandbox en shellant vers le CLI `docker`, et construit les
> images. La construction de la commande `docker run` est une **fonction pure**,
> testable sans daemon.

## Rôle & responsabilités

Ce package fournit :

- l'interface `Runner` (couture DIP) : `Run(ctx, spec, stdout, stderr)` +
  `Build(ctx, contextDir, tag, dockerfile, buildArgs)` ;
- `CLI`, l'implémentation adossée au binaire `docker` réel (`NewCLI()`) ;
- `RunArgs(spec RunSpec) []string`, le constructeur **pur** de la commande.

Il **ne connaît rien** de Claude : ni `--dangerously-skip-permissions`, ni
`--setting-sources`, ni le modèle, ni le prompt, ni le choix d'image. Ces
éléments sont décidés par l'appelant (`internal/runner`) et passés via
`RunSpec.Image` / `RunSpec.Args` / `RunSpec.Entrypoint`. Le non-root n'est pas
imposé ici mais par le `USER` des Dockerfiles.

## Flux principaux

`RunArgs` produit exactement, dans l'ordre :

1. `run`, `--rm`
2. `-v <WorkDir>:/work`
3. `-w /work`
4. si `CredsFile` non vide : `-v <CredsFile>:/benchy/creds/.credentials.json:ro`
5. pour chaque clé d'env (triée) : `-e k=v`
6. si `Entrypoint` non vide : `--entrypoint <Entrypoint>`
7. `<Image>`
8. `<Args…>`

`CLI.Run` exécute `docker <RunArgs>` et, en cas d'échec, enrichit l'erreur avec la
**queue** de stderr (au plus `maxTail = 512` octets, aplatie sur une ligne) pour
transformer un opaque `exit status 125` en diagnostic. `CLI.Build` assemble
`docker build -t tag [-f …] [--build-arg …] contextDir`, arguments triés.

## Dépendances & cibles

Bibliothèque standard + binaire `docker` de l'hôte. Consommé par
`internal/runner` (runs Claude et rejeu des checks) et par `cmd/benchy build-image`.

## Invariants & pièges

- **`--rm` toujours** : les conteneurs sandbox sont éphémères.
- **Creds montés en lecture seule** (`:ro`) et seulement s'ils sont fournis.
- **Isolation par omission** : seuls `WorkDir` et (optionnellement) le fichier de
  creds sont montés — aucun `~/.claude`, réglage ou skill de l'hôte. L'isolation
  des réglages est complétée par le runner via `--setting-sources project,local` —
  voir [ADR-0003](../decisions/0003-isolation-sandbox-et-reglages.md).
- **Ordre d'arguments déterministe** (env et build-args triés).
- **Queue d'erreur bornée** à 512 octets.
- Il n'existe **pas** de sélecteur d'image dans ce package : l'image est une
  simple chaîne (`RunSpec.Image`) choisie par l'appelant.

## Cas de test associés

`docker_test.go` : `TestRunArgs`, `TestRunArgsWithoutCreds`,
`TestRunArgsWithEntrypoint`.
`tailwriter_internal_test.go` : `TestTailWriterKeepsLastBytesFlattened`,
`TestTailWriterCapsRetainedBytes`, `TestTailWriterEmpty`.
(`CLI.Run` / `CLI.Build` / `NewCLI` touchent le daemon et ne sont pas testés
unitairement.)
