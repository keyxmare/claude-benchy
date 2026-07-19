---
titre: internal/server — dashboard web
public: dev
sources:
  - internal/server/server.go
  - internal/server/jobs.go
  - internal/server/form.go
  - internal/server/browse.go
  - internal/server/history.go
  - internal/server/reuse.go
  - internal/server/apply.go
  - internal/server/exportconfig.go
---

# internal/server — dashboard web

> Dashboard local (bibliothèque standard), qui réutilise le pipeline de la CLI :
> formulaire de banc, progression live (SSE), sortie par agent, historique,
> reprise de config, application du diff, export de config, pickers de fichiers
> côté serveur.

## Rôle & responsabilités

Aucune dépendance runtime hors bibliothèque standard (seul `gopkg.in/yaml.v3` est
importé). Réutilise `runner.Run`, `runner.Reload`, `report.Write`. Les templates
(`templates/*.html`) et les assets (`static/style.css`) sont embarqués via
`//go:embed`. Posture **localhost par défaut** : les pickers listent librement le
système de fichiers de l'hôte et le sandbox monte les creds — voir
[ADR-0008](../decisions/0008-dashboard-stdlib-localhost.md).

## Flux principaux

Routes (mux `net/http` méthode+motif, Go 1.22) :

| Route | Rôle |
|---|---|
| `GET /{$}` | Dashboard avec formulaire vierge. |
| `GET /new` | Préremplit le formulaire : `?import=` importe un bench.yaml, `?from=` reprend un run passé. |
| `POST /run` | Formulaire → spec → lance le job → 303 vers `/runs/{id}`. |
| `GET /runs/{id}` | Page de progression live d'un job. |
| `GET /runs/{id}/events` | Flux SSE des événements `log`/`agent`/`done`. |
| `POST /runs/{id}/stop` | Annule le contexte du job. |
| `GET /report` | Recharge et rend le rapport HTML d'un run (`?dir=`). |
| `GET /transcript` | Rejoue le `transcript.jsonl` d'un agent en feed rendu (`?dir=`). |
| `POST /apply` | `git apply` du `diff.patch` d'un run sur l'app (non committé). |
| `GET /config-files` | Liste JSON des fichiers du bundle d'un run. |
| `POST /export-config` | Copie les fichiers de bundle sélectionnés dans l'app. |
| `GET /browse` | Listing JSON d'un dossier hôte pour les pickers (`?path=`, `?mode=`). |
| `GET /file` | Sert un fichier texte hôte à l'aperçu (`?path=`). |
| `GET /static/` | Assets embarqués. |

- **Formulaire → spec** (`form.go`) : les lignes config/check arrivent en slices
  parallèles alignées par index, réassemblées de façon bornée ; lignes vides
  ignorées ; `runs`/`concurrency` parsés (blanc→0, non numérique→erreur) ;
  `retries` tri-état (blanc→nil, `0`/`N` préservés). La validation de fond est
  déléguée à `spec.Build`. Le `bench.yaml` persisté est sérialisé **avant**
  `spec.Build` pour rester relatif et rejouable.
- **Live** (`jobs.go`) : un `jobManager` réserve un dossier horodaté, lance
  `runner.Run` en goroutine ; les événements sont diffusés via un canal
  fermé-remplacé (`signalLocked`) ; `handleEvents` sert du SSE.
- **Historique** (`history.go`) : `scanHistory` reconnaît un dossier de run par la
  présence de `bench.json`.
- **Reprise** (`reuse.go`) : trois niveaux de fidélité — `bench.yaml` propre du
  run, sinon `bench.yaml` ancêtre (≤ 6 niveaux), sinon reconstruction depuis les
  artefacts.
- **Appliquer / exporter** (`apply.go`, `exportconfig.go`) : écritures **non
  committées** ; rejets de chemins `..`/absolus.

## Dépendances & cibles

`internal/runner`, `internal/report`, `internal/docker`, `internal/spec`,
`gopkg.in/yaml.v3`. Lancé par `cmd/benchy serve`.

## Invariants & pièges

- **Rejet du path traversal** là où il compte : `/transcript`, `/report`,
  `/apply`, `/export-config`, `/config-files`, `/new?from=` sont confinés à la
  racine ; `/browse` et `/file` sont **délibérément** non confinés (posture
  localhost). Voir [ADR-0008](../decisions/0008-dashboard-stdlib-localhost.md).
- **Jamais de commit** : `apply` et `export` laissent les changements non stagés.
- **L'image du banc l'emporte** sur l'image par défaut du serveur (`chooseImage`).
- L'état des jobs en mémoire est perdu au redémarrage, mais les artefacts restent
  listables via l'historique.

## Cas de test associés

`server_test.go` : `TestDashboardRenders`,
`TestRunLaunchesJobAndStreamsToCompletion`, `TestTranscriptRejectsPathTraversal`,
`TestNewPrefillsFormFromRunConfig`, `TestRunRerendersFormOnValidationError`,
`TestBrowseListsDirectories`, `TestFileServesTextAndRejectsDir`,
`TestReuseRecoversBundleFromAncestorBench`, `TestReportRejectsPathTraversal`.
`form_test.go` : `TestFormSpecDropsBlankRowsAndParsesLists`,
`TestFormCarriesKeepBaseConfig`, `TestFormRetriesRoundTrip`,
`TestChooseImagePrefersBenchImage`, `TestFormSpecRejectsInvalidNumber`,
`TestBenchDocMarshalsTidyYAML`.
`import_test.go` : `TestNewImportsBenchFileWithAbsolutePaths`,
`TestNewImportErrors`, `TestNewWithoutParamsRendersDefaultForm`,
`TestNewFromInvalidDirIsRejected`, `TestResolveExistingFile`.
`apply_test.go` : `TestApplyAppliesPatchToProject`, `TestApplyErrors`.
`exportconfig_test.go` : `TestConfigFilesLists`, `TestConfigFilesErrors`,
`TestConfigFilesRejectsBadArtifact`, `TestExportConfigCopiesSelectedFiles`,
`TestExportConfigSingleFileMessage`, `TestExportConfigErrors`.
