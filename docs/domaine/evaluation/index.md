# Contexte métier — évaluation

> *Juger*. Vocabulaire et règles de l'évaluation de l'adéquation à l'attendu.

## Glossaire local

- **Évaluation (`evaluate`)** — le bloc optionnel qui, en fin de run, mesure la
  qualité des productions.
- **Rubric** — la liste de critères qualitatifs jugés par le LLM.
- **Check déterministe** — une vérification rejouée dans le sandbox : une commande
  qui doit sortir en `0` (`run`) ou un fichier qui doit exister (`file`).
- **Juge (juge LLM)** — un run Claude dédié qui lit les productions exploitables et
  attribue un niveau par critère.
- **Critère** — un élément du rubric.
- **Niveau** — l'appréciation d'un critère : respecté / partiel / non.
- **Score** — la note 0–100 d'une config, **calculée** depuis les niveaux
  (respecté = 1, partiel = 0,5, non = 0).
- **Dispersion** — l'écart min–max des scores des runs d'une config (variance).
- **Classement** — l'ordre des configs par score moyen décroissant.
- **Recommandation** — le meilleur compromis désigné par l'analyse déterministe.
- **Efficacité** — le bilan comparatif des axes coût / durée / tours / appels
  d'outils.

## Règles

- **RG-eval-01 — Déclenchement et validité.** L'évaluation ne tourne que si un
  rubric **ou** des checks sont déclarés ; un check doit fixer `run` **ou** `file`.
  Source : `internal/spec/spec.go` (`Evaluate.Enabled`, `validate`). Tests :
  `TestLoadEvaluate`, `TestLoadEvaluateInvalidCheck` (`internal/spec/spec_test.go`).
- **RG-eval-02 — Le juge ignore les runs non exploitables.** Il n'examine que les
  runs `OK()` et est sauté si aucun run n'est OK. Source :
  `internal/runner/judge.go` (`buildJudgePrompt`), `internal/runner/runner.go`.
  Tests : `TestBuildJudgePromptEmbedsOKRunsAndTruncates`,
  `TestParseJudgeToleratesFencesAndComputesScoreRanking`.
- **RG-eval-03 — Scoring déterministe.** Le score est calculé en Go depuis les
  niveaux par critère, jamais fourni par le modèle. Source :
  `internal/report/report.go` (`ScoreFromCriteria`), `internal/runner/judge.go`
  (`parseJudge`, `rankByScore`). Tests :
  `TestParseJudgeToleratesFencesAndComputesScoreRanking`,
  `TestEvalByConfigAggregatesRunsAndExcludesDegenerate`.
  Voir [ADR-0006](../../architecture/decisions/0006-juge-llm-scoring-deterministe.md).
- **RG-eval-04 — Agrégation par config.** L'évaluation est agrégée par config :
  score moyen des runs jugés + dispersion (min–max) ; tri par score moyen
  décroissant. Source : `internal/report/report.go` (`EvalByConfig`,
  `aggregateConfig`). Tests : `TestEvalByConfigAggregatesRunsAndExcludesDegenerate`,
  `TestEvalByConfigNilWithoutEvaluation`.
- **RG-eval-05 — Modèle juge par défaut.** Le modèle juge vaut, par défaut, le
  modèle du banc. Source : `internal/spec/spec.go` (`resolve`). Tests :
  `TestLoadEvaluate`.

## Fonctionnalités rattachées

- [Checks déterministes et juge LLM](../../fonctionnel/evaluation/checks-et-juge.md)
- [Comparaison, agrégation et efficacité](../../fonctionnel/evaluation/comparaison-et-efficacite.md)
- [Re-générer un rapport](../../fonctionnel/evaluation/regenerer-un-rapport.md)
