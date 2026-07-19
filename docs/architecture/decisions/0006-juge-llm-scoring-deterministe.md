# ADR-0006 — Juge LLM à scoring déterministe

## Statut

Accepté

## Contexte

L'évaluation qualitative (rubric) demande un jugement de LLM. Mais laisser le
modèle produire lui-même un score chiffré rendrait le classement instable et peu
comparable d'un run à l'autre. Il faut aussi borner le coût de l'appel juge et le
rendre robuste à un JSON malformé.

## Décision

Le juge est un run Claude dédié (sans `--setting-sources` ni
`--append-system-prompt`) qui lit les runs **exploitables** (`OK()`) et, pour
chaque critère du rubric, rend un **niveau** (respecté / partiel / non), pas un
score. Le **score est calculé en Go** (`report.ScoreFromCriteria` : respecté = 1,
partiel = 0,5, non = 0, moyenne × 100), puis les configs sont classées
(`rankByScore`). Chaque diff embarqué est tronqué à `maxDiffBytes = 6000`. En cas
de JSON invalide, une relance corrective unique est tentée
(`parseJudge`/`extractJSONObject` tolèrent fences et prose environnante).

## Conséquences

- Les scores sont reproductibles et comparables (dérivés de critères, pas du
  modèle).
- Le coût de l'appel juge est borné (diffs tronqués, un seul appel + au plus une
  relance).
- Un échec du juge n'interrompt jamais le banc : l'évaluation est simplement
  omise.
- Les runs dégénérés/échoués sont exclus du prompt du juge (cf.
  [ADR-0001](0001-runs-sans-effet-detection-relance-exclusion.md)).
