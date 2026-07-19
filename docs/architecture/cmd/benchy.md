---
titre: cmd/benchy — point d'entrée CLI
public: dev
sources:
  - cmd/benchy/main.go
---

# cmd/benchy — point d'entrée CLI

> La ligne de commande de benchy : elle câble les sous-commandes au pipeline
> (`runner.Run`, `runner.Reload`, `report.Write`) et au dashboard.

## Rôle & responsabilités

`main()` appelle `run(os.Args[1:])`, monte un contexte annulable sur
`os.Interrupt`, puis dispatche sur la sous-commande. Constantes :
`defaultAddr = 127.0.0.1:80`, `defaultImage = claude-benchy:latest`,
`dockerContextDir = build/docker`.

## Sous-commandes

| Commande | Effet |
|---|---|
| `run [--image IMG] <bench.yaml>` | `spec.Load` → `runner.Run` (sortie `<output>/<horodatage>`) → copie du `bench.yaml` source dans le dossier de résultats → `report.Write`. `--image` explicite l'emporte sur `sandbox.image`, sinon défaut. |
| `new --from <bench.yaml> [--output F]` | `spec.Import` réécrit les chemins en absolu ; écrit sur stdout ou dans `--output`. |
| `report <results-dir>` | `runner.BenchInfo` + `runner.Reload` reconstruisent le rapport depuis les artefacts, puis `report.Write` — sans appel API. |
| `serve [--addr A] [--root D] [--image IMG]` | `server.New` + `http.Server` ; arrêt gracieux (5 s) sur annulation du contexte. |
| `build-image [--tag T] [--context D] [--dockerfile F] [--claude-version V]` | `docker.CLI.Build` ; `--claude-version` passe le build-arg `CLAUDE_VERSION`. |
| `-h` / `--help` / `help` | Affiche l'aide. |

## Dépendances & cibles

`internal/spec`, `internal/runner`, `internal/report`, `internal/docker`,
`internal/server`. Compilé en binaire natif via `make build`.

## Invariants & pièges

- `run` persiste le `bench.yaml` source dans le dossier de résultats
  (reproductibilité, reprise depuis le dashboard).
- `--image` explicite (détecté via `fs.Visit`) prend le pas sur l'image du banc.
- Le texte d'aide `usage()` ne liste pas le flag `--dockerfile` de `build-image`,
  qui existe pourtant.

## Cas de test associés

Le package `main` n'a pas de tests dédiés ; son comportement est couvert
indirectement par les tests de `internal/runner`, `internal/report` et
`internal/server` qu'il orchestre.
