---
titre: Lancer et suivre un banc
public: tous
sources:
  - internal/runner/runner.go
  - cmd/benchy/main.go
---

# Lancer et suivre un banc

> Exécuter la comparaison : pour chaque config et chaque run, benchy prépare une
> copie isolée, lance Claude en autonomie, puis capture le résultat — en affichant
> une progression live.

## User story

En tant qu'utilisateur, je veux lancer mon banc d'une commande et voir sa
progression, afin d'obtenir un rapport comparatif reproductible sans piloter
chaque exécution à la main.

## Le déroulé

`benchy run <bench.yaml>` exécute chaque job (config × run). Le pipeline par job :

1. prépare un workspace isolé (copie de l'app + bundle, baseline git) ;
2. lance Claude dans le bac à sable (transcript `stream-json`) ;
3. parse les métriques puis capture le diff.

Les jobs tournent en parallèle dans la limite de `concurrency`. Chaque exécution
écrit ses artefacts sous `<output>/<horodatage>/`, affiche début/fin de chaque
config (durée, coût, diff) et le chemin du rapport HTML en fin de run.

## Les artefacts produits

```text
results/<horodatage>/
├── <config>/                  # ou <config>/run-N/ si runs > 1
│   ├── workspace/             # copie isolée après passage de Claude
│   ├── diff.patch             # modifications apportées à l'app
│   ├── transcript.jsonl       # flux stream-json complet
│   ├── result.json            # métriques (coût, tokens, tours, durée)
│   ├── checks.json            # résultats des checks (si evaluate)
│   └── stdout.log             # sortie du conteneur
├── bench.json                 # prompt + app (indexation historique)
├── bench.yaml                 # config du banc (persistée)
├── evaluation.json            # verdict du juge (si evaluate)
├── judge-transcript.jsonl     # transcript du juge (si evaluate)
├── report.md
└── report.html
```

## Critères d'acceptation

- **Given** un banc valide, **When** on lance `benchy run`, **Then** chaque job
  produit `transcript.jsonl`, `result.json` et `diff.patch`, et un `report.html`
  est écrit.
- **Given** `concurrency: N`, **When** on lance, **Then** au plus N runs Docker
  tournent en parallèle.
- **Given** l'échec de préparation d'un run (bundle manquant), **When** on lance,
  **Then** l'erreur est enregistrée dans le rapport et le banc n'est pas
  interrompu.
- **Given** un run réussi, **When** il se termine, **Then** une ligne « ✓ »
  résume durée, coût et diff.

## Cas non triviaux & limites

- Un job en échec dur n'interrompt pas les autres : l'erreur est consignée.
- Un run peut être *sans effet* et déclencher une relance — voir
  [Runs sans effet](runs-sans-effet.md).
- Le `bench.yaml` source est copié dans le dossier de résultats pour rejeu et
  reprise.

## Cas de test associés

`internal/runner/runner_test.go` : `TestRunEndToEndWithFake`,
`TestRunRecordsPrepareFailure`, `TestCollectFilesReadsTouchedFilesOnly`,
`TestPostImagePath`.
