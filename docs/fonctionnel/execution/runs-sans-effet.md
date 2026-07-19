---
titre: Runs sans effet et relances
public: tous
sources:
  - internal/runner/runner.go
  - internal/report/report.go
---

# Runs sans effet et relances

> Un run peut « réussir » sans rien produire. benchy détecte ces faux succès, les
> relance, et les écarte des classements pour ne pas fausser les moyennes.

## User story

En tant qu'utilisateur, je veux que les runs qui n'ont rien produit ne polluent
pas mes résultats, afin que les scores et les classements reflètent le vrai
travail des configurations.

## Le problème et la parade

Un run peut se terminer sans erreur mais sans aucun appel d'outil, ou avec un diff
vide — par exemple quand le modèle écrit un appel de sous-agent en texte au lieu
de l'exécuter. C'est un **faux succès** : un 0/100 parasite pour les moyennes, un
run à ~0 s / ~0 $ pour l'efficacité.

benchy détecte ces runs **sans effet** et les **relance** jusqu'à `retries` fois
(défaut 2). S'il reste sans effet, le run est marqué « sans effet » et **écarté**
de toutes les agrégations (score, efficacité, juge), tout en restant visible dans
la comparaison et le détail. `retries: 0` désactive la relance.

Voir [ADR-0001](../../architecture/decisions/0001-runs-sans-effet-detection-relance-exclusion.md)
et les règles [RG-exec-01 à RG-exec-03](../../domaine/execution/index.md#règles).

## Critères d'acceptation

- **Given** un run sans erreur mais sans appel d'outil ou à diff vide, **When** il
  se termine, **Then** il est considéré « sans effet » (dégénéré).
- **Given** `retries: 2` et une première tentative sans effet, **When** le run
  s'exécute, **Then** il est relancé jusqu'à produire du travail (au plus 3
  tentatives), et le résultat retenu est celui de la tentative réussie.
- **Given** `retries: 0` et un run toujours sans effet, **When** il se termine,
  **Then** il n'est pas relancé, est marqué « sans effet » et n'est pas OK.
- **Given** un run resté sans effet, **When** le rapport est agrégé, **Then** il
  est exclu des scores, de l'efficacité et du juge.
- **Given** une erreur dure (non « sans effet »), **When** elle survient, **Then**
  le run n'est pas relancé.

## Cas non triviaux & limites

- Dégénéré et OK sont mutuellement exclusifs : un run dégénéré n'est jamais OK.
- Chaque relance repart d'un workspace neuf.
- Un `headlessSystemPrompt` est appliqué à chaque config pour éviter qu'une
  session s'arrête en posant une question — cause fréquente de run sans effet.

## Cas de test associés

`internal/runner/runner_test.go` :
`TestRunRetriesDegenerateRunUntilItProducesWork`,
`TestRunRecordsPersistentlyDegenerateRun`.
`internal/report/report_test.go` : `TestDegenerateAndOK`,
`TestEvalByConfigAggregatesRunsAndExcludesDegenerate`.
