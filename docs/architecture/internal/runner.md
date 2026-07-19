---
titre: internal/runner — orchestration du banc
public: dev
sources:
  - internal/runner/runner.go
  - internal/runner/judge.go
  - internal/runner/reload.go
---

# internal/runner — orchestration du banc

> Le cœur du projet : ordonnance les jobs (config × run) en parallèle borné,
> détecte et relance les runs sans effet, rejoue les checks déterministes, fait
> tourner le juge LLM, et sait reconstruire un rapport depuis les artefacts.

## Rôle & responsabilités

- `Run(ctx, s, outputRoot, generatedAt, opts)` exécute tout le banc, écrit les
  artefacts et renvoie le `report.Report` agrégé ; un échec de run individuel est
  **enregistré** dans le rapport, il n'interrompt pas le banc.
- `Reload(outputRoot, app, prompt, generatedAt)` reconstruit un rapport depuis les
  artefacts sur disque, **sans** nouvel appel API.
- `BenchInfo(outputRoot)` relit le prompt et l'app depuis `bench.json`.
- `Options{Image, Docker, Log, Agent}` : `Log` reçoit les lignes de progression,
  `Agent` les mises à jour par agent (config × run) via `AgentEvent`, dont le
  cycle de vie est typé `Status` (`running`/`done`/`failed`/`degenerate`,
  vocabulaire partagé avec le dashboard) ; les deux peuvent être appelés en
  concurrence.

## Flux principaux

**Ordonnancement** (`runner.go`) : `expand(s)` produit la liste des jobs (boucle
`config × run`, chaque job portant un `index` monotone). Un sémaphore de capacité
`s.Concurrency` (`chan struct{}`) borne la parallélisation ; une `WaitGroup`
attend tous les jobs ; les résultats sont écrits par `index` dans une slice
pré-dimensionnée (pas de collision).

**Pipeline par job** (`attemptRun`) : `RemoveAll(workspaceDir)` (départ propre
pour une relance) → `workspace.Prepare(App, bundle, workspaceDir,
KeepBaseConfig)` → `runClaude` → `diffcap.Capture`. `runClaude` construit le
`docker.RunSpec` (env `DISABLE_AUTOUPDATER=1`) avec `claudeArgs` : `-p <prompt>
--output-format stream-json --verbose --dangerously-skip-permissions
--setting-sources project,local --append-system-prompt <headlessSystemPrompt>
--model <model>`, tee le transcript vers `claude.NewLiveWriter` tout en
persistant le flux brut, puis `claude.Parse` écrit `result.json`.

**Runs sans effet** : la définition vit dans `report.RunReport.Degenerate()` — un
run sans erreur mais avec `ToolUses == 0` **ou** `FilesChanged == 0`. La boucle
tente `1 + RetryCount()` fois ; elle s'arrête sur une erreur dure (non relancée)
ou au premier résultat non dégénéré. Un run dégénéré persistant est marqué
« sans effet » et **écarté des agrégations**. Voir
[ADR-0001](../decisions/0001-runs-sans-effet-detection-relance-exclusion.md).

**Juge** (`judge.go`) : `buildJudgePrompt` embarque les runs **exploitables**
seulement (`run.OK()`), chaque diff tronqué à `maxDiffBytes = 6000`. `runJudge`
lance un run Claude dédié (sans `--setting-sources` ni `--append-system-prompt`).
`parseJudge` extrait le premier objet JSON (tolère fences et prose), une relance
corrective en cas de JSON invalide. Le **score est calculé en Go**
(`report.ScoreFromCriteria`), jamais fourni par le modèle ; `rankByScore` classe
les configs. Voir
[ADR-0006](../decisions/0006-juge-llm-scoring-deterministe.md).

**Reload** (`reload.go`) : `filepath.WalkDir` repère un dossier de run par la
présence de `diff.patch`, reconstruit chaque `RunReport` depuis `meta.json`,
`result.json`, `diff.patch` (stats recalculées via `patchStats`, fichiers
touchés via `collectFiles`) et `checks.json`, puis charge `evaluation.json`. Voir
[ADR-0007](../decisions/0007-reconstruction-rapport-sans-api.md).

## Dépendances & cibles

Dépend de `internal/spec`, `internal/workspace`, `internal/docker`,
`internal/claude`, `internal/diffcap`, `internal/report`. Injecté avec un
`docker.Runner` (fake en test).

## Invariants & pièges

- **Dégénéré ⇒ non OK** (mutuellement exclusifs) ; les runs dégénérés/échoués
  sont exclus du prompt du juge, donc du scoring/classement ; le juge est sauté si
  aucun run n'est OK.
- Une erreur dure **n'est pas** relancée (seuls les runs dégénérés le sont) ;
  chaque relance repart d'un workspace neuf.
- Les checks « run » s'exécutent dans le sandbox via `sh -c` (pas `-lc`, pour ne
  pas réinitialiser le PATH et perdre le toolchain) ; les checks « file » stat un
  chemin ; ils ne tournent que pour les runs OK.
- Un `headlessSystemPrompt` est appliqué à **chaque** config (neutralité) pour
  éviter qu'une session s'arrête en posant une question — ce qui produirait
  justement un run sans effet.
- Un échec du juge n'interrompt jamais le banc (l'évaluation est simplement
  omise).

## Cas de test associés

`runner_test.go` : `TestRunEndToEndWithFake`,
`TestRunWithEvaluationJudgesAndChecks`, `TestRunEvaluationRecordsFailingCheck`,
`TestPostImagePath`, `TestCollectFilesReadsTouchedFilesOnly`,
`TestRunRetriesDegenerateRunUntilItProducesWork`,
`TestRunRecordsPersistentlyDegenerateRun`, `TestRunRecordsPrepareFailure`,
`TestClaudeArgsForceNonInteractive`.
`judge_test.go` : `TestBuildJudgePromptEmbedsOKRunsAndTruncates`,
`TestParseJudgeToleratesFencesAndComputesScoreRanking`,
`TestParseJudgeRejectsNonJSON`.
