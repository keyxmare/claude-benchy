---
titre: Comparaison, agrégation et efficacité
public: tous
sources:
  - internal/report/report.go
  - internal/report/templates.go
---

# Comparaison, agrégation et efficacité

> Le rapport réunit les métriques, l'évaluation agrégée par config, un bilan
> d'efficacité et une comparaison côte à côte du code produit.

## User story

En tant qu'utilisateur, je veux un rapport qui compare mes configs sur un même
plan — qualité, coût, code produit — afin de décider laquelle adopter.

## Ce que contient le rapport

- **Métriques** par run (coût, tokens, tours, durée).
- **Évaluation agrégée par config** : chaque run exploitable est noté, puis la
  config reçoit le **score moyen** de ses runs et sa **dispersion** (min–max), qui
  mesure la divergence entre runs. Le tri se fait par score moyen décroissant.
- **Bilan d'efficacité** : points forts / faibles de chaque config (coût,
  rapidité, périmètre traité) déduits des mesures, avec désignation d'un meilleur
  compromis et, si une config `vanilla`/`baseline` existe, des écarts chiffrés.
- **Comparaison côte à côte** : on choisit deux configs et on lit, fichier par
  fichier, le code produit de part et d'autre, lignes divergentes surlignées.

Voir les règles [RG-eval-04](../../domaine/evaluation/index.md#règles) et
[RG-exec-03](../../domaine/execution/index.md#règles) (exclusion des runs sans
effet).

## Critères d'acceptation

- **Given** plusieurs runs par config avec évaluation, **When** le rapport est
  agrégé, **Then** chaque config affiche un score moyen et sa dispersion, triés du
  meilleur au moins bon.
- **Given** un run dégénéré parmi les runs d'une config, **When** l'agrégation a
  lieu, **Then** il est exclu du score de la config.
- **Given** au moins deux runs OK avec des mesures différentes, **When** le bilan
  d'efficacité est calculé, **Then** les extrêmes (coût/durée/tours/outils au plus
  bas/haut) sont désignés.
- **Given** aucun run réussi, **When** le rapport est produit, **Then** il affiche
  « Aucun run réussi » plutôt qu'un classement vide.
- **Given** deux configs sélectionnées, **When** on ouvre la comparaison, **Then**
  le code de chaque fichier touché est aligné et les lignes divergentes
  surlignées.

## Cas non triviaux & limites

- L'agrégation choisit un **run représentatif** (score le plus proche de la
  moyenne) pour le verdict et les notes.
- Un seul run par config déclenche une réserve de variance dans les
  recommandations.
- La comparaison côte à côte est calculée **dans le navigateur** à partir des
  données injectées ; le rapport HTML reste autoportant (voir
  [ADR-0010](../../architecture/decisions/0010-rapport-html-autoportant.md)).

## Cas de test associés

`internal/report/report_test.go` : `TestAnalysisRanksAndRecommends`,
`TestEvalByConfigAggregatesRunsAndExcludesDegenerate`,
`TestEvalByConfigNilWithoutEvaluation`, `TestAnalysisWithNoSuccessfulRun`,
`TestResultHTMLCollapsesSoftWraps`, `TestGolden`, `TestParseChanges`,
`TestChangeSym`, `TestFileTreeHTML`.
