---
titre: Checks déterministes et juge LLM
public: tous
sources:
  - internal/runner/judge.go
  - internal/runner/runner.go
  - internal/spec/spec.go
---

# Checks déterministes et juge LLM

> Mesurer l'adéquation à l'attendu de deux façons complémentaires : des
> vérifications déterministes rejouées dans le sandbox, et un juge LLM qui note
> les productions selon un rubric.

## User story

En tant qu'utilisateur, je veux évaluer automatiquement si chaque config répond à
la tâche, afin de comparer objectivement leurs productions au-delà du coût.

## Deux mécanismes

Le bloc `evaluate` (facultatif) déclenche, en fin de run :

- les **checks** : des vérifications déterministes rejouées dans le bac à sable
  contre le workspace de chaque config — une commande qui doit sortir en `0`
  (`run`) ou un fichier qui doit exister (`file`). L'image du sandbox doit donc
  embarquer le toolchain nécessaire ;
- le **rubric** : les critères qualitatifs qu'un **juge LLM** applique en lisant
  les diffs et résumés des runs **exploitables**, en attribuant un niveau par
  critère (respecté / partiel / non).

Le **score** est calculé en Go depuis ces niveaux (respecté = 1, partiel = 0,5,
non = 0), jamais fourni par le modèle. Voir
[ADR-0006](../../architecture/decisions/0006-juge-llm-scoring-deterministe.md) et
les règles [RG-eval-01 à RG-eval-03, RG-eval-05](../../domaine/evaluation/index.md#règles).

## Critères d'acceptation

- **Given** un `evaluate` avec rubric et checks, **When** un run OK se termine,
  **Then** les checks sont rejoués (résultats dans `checks.json`) et le juge note
  le run.
- **Given** un check `run` qui sort en `0`, **When** il est rejoué, **Then** il
  passe ; s'il sort non nul, il échoue avec sa sortie.
- **Given** des runs dégénérés/échoués, **When** le juge est construit, **Then**
  ils sont exclus de son prompt.
- **Given** une réponse du juge non-JSON, **When** elle est analysée, **Then** une
  relance corrective est tentée puis, à défaut, l'évaluation est omise sans
  interrompre le banc.
- **Given** un `evaluate` sans `model`, **When** le banc est chargé, **Then** le
  modèle juge vaut celui du banc.

## Cas non triviaux & limites

- Le juge est un appel Claude supplémentaire ; les diffs embarqués sont tronqués
  (`maxDiffBytes = 6000`) pour borner le coût.
- Les checks « run » s'exécutent via `sh -c` (pas un shell de login) pour ne pas
  perdre le PATH du toolchain.
- Un check invalide (ni `run` ni `file`) rend le banc invalide.
- Le résultat est persisté (`evaluation.json`, `checks.json`) et réaffiché tel
  quel par `benchy report`, sans nouvel appel.

## Cas de test associés

`internal/runner/runner_test.go` : `TestRunWithEvaluationJudgesAndChecks`,
`TestRunEvaluationRecordsFailingCheck`.
`internal/runner/judge_test.go` : `TestBuildJudgePromptEmbedsOKRunsAndTruncates`,
`TestParseJudgeToleratesFencesAndComputesScoreRanking`,
`TestParseJudgeRejectsNonJSON`.
`internal/spec/spec_test.go` : `TestLoadEvaluate`, `TestLoadEvaluateInvalidCheck`.
