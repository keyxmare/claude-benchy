# ADR-0007 — Reconstruction du rapport sans appel API

## Statut

Accepté

## Contexte

Faire évoluer la mise en page du rapport (ou corriger un rendu) ne doit pas
imposer de relancer les runs Claude, qui coûtent du temps et de l'argent. Les
artefacts d'un banc (transcripts, diffs, métriques, évaluation) suffisent à
reconstruire le rapport.

## Décision

`runner.Reload` reconstruit un `report.Report` **uniquement** depuis les artefacts
sur disque, sans aucun appel Docker ni API. Un dossier de run est reconnu par la
présence de `diff.patch` ; chaque `RunReport` est reconstitué depuis `meta.json`,
`result.json`, `diff.patch` (stats recalculées via `patchStats`, fichiers touchés
via `collectFiles`) et `checks.json` ; l'évaluation est relue depuis
`evaluation.json`. Exposé par `benchy report <results-dir>` et par la route
`/report` du dashboard.

## Conséquences

- Les rapports se régénèrent gratuitement après une évolution des templates.
- Le verdict du juge et les checks sont persistés (`evaluation.json`,
  `checks.json`) et réaffichés tels quels — pas de re-jugement.
- Les statistiques de diff sont recalculées depuis le patch (source unique :
  l'artefact), au lieu d'être relues d'un cache séparé.
