---
titre: internal/report — agrégation et rapport
public: dev
sources:
  - internal/report/report.go
  - internal/report/templates.go
---

# internal/report — agrégation et rapport

> Agrège les résultats par run en `report.md` + `report.html`, avec agrégation
> par config (score moyen + dispersion), bilan d'efficacité et comparaison
> côte à côte fichier par fichier.

## Rôle & responsabilités

- `Write(dir, r)` crée `report.md` (via `WriteMarkdown`) puis `report.html` (via
  `WriteHTML`), tous deux à partir des mêmes données.
- Porte le modèle de données du rapport : `Report`, `RunReport`, `Evaluation`,
  `ConfigScore`, `Analysis`, etc.
- Définit les prédicats **centraux** `RunReport.Degenerate()` et `RunReport.OK()`,
  ainsi que le calcul de score `ScoreFromCriteria`.
- Modélise les valeurs porteuses d'invariant en **value objects** : `Level` (le
  niveau d'un critère, construit par `NormalizeLevel`, qui porte son poids de
  score, sa classe CSS et son glyphe) et `Rank` (le rang d'efficacité
  best/mid/worst d'une config sur un axe, rendu via le visuel de `Level`).
- Le rapport HTML est autoportant (design **Gazoline** de Motoblouz, CSS inline).

## Flux principaux

**Agrégation par config** (`EvalByConfig`) : renvoie nil si aucune évaluation ;
sinon groupe les runs par config (ordre de première apparition), ne garde que les
runs `OK()` **jugés** (critères non vides), calcule `mean/min/max`, choisit un
**run représentatif** (score le plus proche de la moyenne, égalité → premier) pour
le verdict et les notes, agrège les niveaux par critère et les checks, puis trie
par score moyen décroissant. La dispersion est exposée par `Spread()` et les
champs `MinScore`/`MaxScore`.

**Analyse déterministe** (`Analysis`, `Efficiency`, `Synthesis`, `recommend`) : sur
les seuls runs `OK()`, compare les axes (coût, durée, tours, appels d'outils, « plus
bas/haut »), désigne le meilleur compromis (rang combiné le plus bas), et si une
config `vanilla`/`baseline` existe, chiffre les écarts et prévient quand des
instructions ajoutées coûtent plus sans élargir le périmètre. Un seul run par
config déclenche une réserve de variance.

**Comparaison côte à côte** : le serveur produit les données
(`filesJSON` → variable JS `BENCHY_FILES`), le navigateur calcule l'alignement
(LCS) et rend les lignes (identique / divergent / gauche / droite). `fileTreeHTML`
rend l'arbre des fichiers touchés de chaque run.

## Dépendances & cibles

`text/template` + `html/template` (templates dans `templates.go`),
bibliothèque standard. La feuille de style et les scripts de la page vivent
dans `static/*.{css,js}`, embarqués par `//go:embed` et **inlinés** à la
génération (le rapport reste un fichier unique autoportant). Consommé par
`cmd/benchy` et `internal/server`.

## Invariants & pièges

- **Dégénéré = faux succès** : run sans erreur mais `ToolUses == 0` **ou**
  `FilesChanged == 0`. `OK()` exige aucune erreur, pas d'erreur Claude, et non
  dégénéré. Voir
  [ADR-0001](../decisions/0001-runs-sans-effet-detection-relance-exclusion.md).
- **Exclusion des dégénérés/échoués** de toutes les agrégations (`EvalByConfig`,
  `Analysis`, `Synthesis`, `Efficiency`, `recommend`). Échoués et dégénérés sont
  listés séparément.
- **Évaluation nil** gérée partout (lookups renvoient des zéros).
- La protection contre le *path traversal* n'est **pas** ici : elle vit dans
  `internal/server` ; le rapport n'échappe que lors de la construction d'URL.
- Le rapport HTML embarque son CSS inline pour rester un fichier unique
  ouvrable — voir [ADR-0010](../decisions/0010-rapport-html-autoportant.md).
- Les golden files se régénèrent via la variable d'environnement `UPDATE_GOLDEN`
  (et non un flag `-update`).

## Cas de test associés

`report_test.go` : `TestAnalysisRanksAndRecommends`, `TestDegenerateAndOK`,
`TestEvalByConfigAggregatesRunsAndExcludesDegenerate`,
`TestEvalByConfigNilWithoutEvaluation`, `TestAnalysisWithNoSuccessfulRun`,
`TestGolden`, `TestApplyButtonOnlyWhenServed`, `TestNormalizeLevel`,
`TestScoreFromCriteria`, `TestLevelWeight`, `TestLevelCSSClass`,
`TestLevelSymbol`, `TestRankCSSClass`, `TestRankSymbol`,
`TestEfficiencyRanksBestMidWorst`, `TestEfficiencyNoSuccessfulRun`.
`report_internal_test.go` : `TestResultHTMLCollapsesSoftWraps`,
`TestParseChanges`, `TestChangeSym`, `TestAggregateLevel`, `TestFileTreeHTML`.
