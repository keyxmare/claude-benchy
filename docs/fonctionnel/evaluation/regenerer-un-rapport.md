---
titre: Re-générer un rapport
public: tous
sources:
  - internal/runner/reload.go
  - cmd/benchy/main.go
---

# Re-générer un rapport

> Reconstruire `report.md` / `report.html` à partir des artefacts déjà présents —
> pratique après une évolution de la mise en page, sans re-payer d'appel API.

## User story

En tant qu'utilisateur, je veux régénérer le rapport d'un banc déjà exécuté, afin
de bénéficier d'une nouvelle mise en page ou de relire les résultats sans relancer
les runs.

## Comment

```sh
./benchy report <output>/<horodatage>
```

benchy relit le prompt et l'app (`bench.json`), reconstruit chaque run depuis ses
artefacts (`meta.json`, `result.json`, `diff.patch`, `checks.json`), recharge
l'évaluation (`evaluation.json`), puis réécrit `report.md` et `report.html` — sans
aucun appel Docker ni API. Le dashboard fait de même via la route `/report`.

Voir [ADR-0007](../../architecture/decisions/0007-reconstruction-rapport-sans-api.md).

## Critères d'acceptation

- **Given** un dossier de résultats existant, **When** on lance `benchy report`,
  **Then** `report.md` et `report.html` sont réécrits sans nouvel appel API.
- **Given** une évaluation persistée, **When** le rapport est reconstruit, **Then**
  le verdict du juge et les checks sont réaffichés tels quels.
- **Given** un dossier de run reconnu par la présence de `diff.patch`, **When** la
  reconstruction a lieu, **Then** ses statistiques de diff sont recalculées depuis
  le patch.

## Cas non triviaux & limites

- Le verdict du juge n'est pas recalculé : il est relu tel quel.
- Un dossier sans `diff.patch` n'est pas considéré comme un dossier de run.

## Cas de test associés

La reconstruction est couverte de bout en bout par la route `/report` du
dashboard : `TestRunLaunchesJobAndStreamsToCompletion`
(`internal/server/server_test.go`), et par les helpers `TestPostImagePath`,
`TestCollectFilesReadsTouchedFilesOnly` (`internal/runner/runner_test.go`).
