# Contexte métier — exécution

> *Lancer*. Vocabulaire et règles de l'exécution isolée et reproductible d'un run.

## Glossaire local

- **Workspace** — la copie isolée et jetable de l'app dans laquelle un run se
  déroule.
- **Baseline** — le commit git initial du workspace (app + bundle), point de
  référence du diff.
- **Sandbox / bac à sable** — le conteneur Docker `--rm` non-root exécutant
  Claude pour un run.
- **Run** — une exécution de Claude pour une config donnée. Une config peut avoir
  plusieurs runs (`runs`).
- **Job** — l'unité d'ordonnancement (une paire config × run) planifiée par le
  runner.
- **Run sans effet (dégénéré)** — un run qui « réussit » sans rien produire :
  aucun appel d'outil ou diff vide. Un *faux succès*.
- **Relance** — la ré-exécution d'un run sans effet (jusqu'à `retries`).
- **Concurrence** — le nombre de runs menés en parallèle (`concurrency`).
- **Isolation des réglages** — le fait que les settings/skills de l'hôte ne
  contaminent pas le test (`--setting-sources project,local`).

## Règles

- **RG-exec-01 — Définition du run sans effet.** Un run est dégénéré s'il n'a pas
  d'erreur mais que `ToolUses == 0` **ou** `FilesChanged == 0`. Source :
  `internal/report/report.go` (`RunReport.Degenerate`, `RunReport.OK`). Tests :
  `TestDegenerateAndOK` (`internal/report/report_test.go`).
- **RG-exec-02 — Relance ciblée.** Un run dégénéré est relancé jusqu'à `retries`
  fois ; une erreur dure n'est jamais relancée ; chaque relance repart d'un
  workspace neuf. Source : `internal/runner/runner.go`. Tests :
  `TestRunRetriesDegenerateRunUntilItProducesWork`,
  `TestRunRecordsPersistentlyDegenerateRun` (`internal/runner/runner_test.go`).
- **RG-exec-03 — Exclusion des agrégations.** Un run resté dégénéré est écarté de
  toutes les agrégations (score, efficacité, juge) mais reste visible dans la
  comparaison et le détail. Source : `internal/report/report.go`,
  `internal/runner/judge.go`. Tests :
  `TestEvalByConfigAggregatesRunsAndExcludesDegenerate`,
  `TestBuildJudgePromptEmbedsOKRunsAndTruncates`.
  Voir [ADR-0001](../../architecture/decisions/0001-runs-sans-effet-detection-relance-exclusion.md).
- **RG-exec-04 — Isolation du bac à sable.** Chaque run tourne dans un conteneur
  jetable non-root ne montant que la copie de l'app (rw) et les creds OAuth (ro) ;
  les réglages/skills de l'hôte sont isolés. Source : `internal/docker/docker.go`
  (`RunArgs`), `internal/runner/runner.go` (`claudeArgs`). Tests : `TestRunArgs`,
  `TestRunArgsWithoutCreds`, `TestClaudeArgsForceNonInteractive`.
  Voir [ADR-0003](../../architecture/decisions/0003-isolation-sandbox-et-reglages.md).
- **RG-exec-05 — Baseline incluant le bundle.** La baseline git inclut le bundle,
  de sorte que le diff ne surface que les changements de Claude. Source :
  `internal/workspace/workspace.go` (`Prepare`, `gitBaseline`). Tests :
  `TestPrepareOverlaysAndCommits`, `TestPrepareGitAppUsesCommittedTree`.
  Voir [ADR-0009](../../architecture/decisions/0009-baseline-git-inclut-le-bundle.md).

## Fonctionnalités rattachées

- [Lancer et suivre un banc](../../fonctionnel/execution/lancer-et-suivre.md)
- [Isolation du bac à sable](../../fonctionnel/execution/isolation-sandbox.md)
- [Runs sans effet et relances](../../fonctionnel/execution/runs-sans-effet.md)
