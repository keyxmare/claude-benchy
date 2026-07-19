# Contexte métier — capture

> *Observer*. Vocabulaire et règles de la capture des productions d'un run.

## Glossaire local

- **Transcript** — le flux `stream-json` complet émis par Claude pendant un run,
  persisté en `transcript.jsonl`.
- **Événement** — une ligne du transcript (`system`, `assistant`, `user`,
  `result`).
- **Métriques (`result.json`)** — le bilan chiffré d'un run : coût, tokens
  (entrée/sortie/cache), tours, durée, appels d'outils, ventilation par outil,
  statut d'erreur.
- **Appel d'outil (`tool_use`)** — l'invocation d'un outil par Claude (Bash, Edit,
  Read…), comptée pour détecter les runs sans effet.
- **Diff / patch (`diff.patch`)** — le diff unifié des changements du workspace
  contre la baseline.
- **Statistiques de diff** — fichiers changés, insertions, suppressions.

## Règles

- **RG-capture-01 — Événement `result` obligatoire.** Un transcript doit contenir
  un événement `result` terminal ; son absence lève une erreur et signale un run
  vide. Source : `internal/claude/stream.go` (`Parse`). Tests : `TestParse`,
  `TestParseNoResult`, `TestParseIgnoresGarbageLines` (`internal/claude/stream_test.go`).
- **RG-capture-02 — Comptage d'outils dérivé du flux.** Le nombre et la
  ventilation des appels d'outils sont comptés sur les blocs `tool_use` du flux,
  pas lus du result event. Source : `internal/claude/stream.go` (`Parse`). Tests :
  `TestParse`.
- **RG-capture-03 — Capture du delta complet.** La capture stage tous les
  changements (`git add -A`, ajouts/modifs/suppressions) et produit le diff + les
  statistiques contre la baseline. Source : `internal/diffcap/diffcap.go`
  (`Capture`, `parseNumstat`). Tests : `TestCaptureChanges`, `TestCaptureNoChanges`
  (`internal/diffcap/diffcap_test.go`).

## Fonctionnalités rattachées

La capture est un maillon interne du pipeline ; ses productions sont exposées par
les fonctionnalités du contexte [évaluation](../evaluation/index.md) et
[pilotage](../../fonctionnel/pilotage/dashboard-web.md) (page transcript,
comparaison côte à côte). Voir [lancer et suivre](../../fonctionnel/execution/lancer-et-suivre.md)
pour la liste des artefacts produits.
