---
titre: Importer et reprendre une config
public: tous
sources:
  - internal/server/reuse.go
  - internal/server/import.go
  - internal/spec/import.go
  - cmd/benchy/main.go
---

# Importer et reprendre une config

> Repartir d'un banc existant ou d'un run passé, sans tout ressaisir : les chemins
> relatifs sont réécrits en absolu pour que le banc soit lançable de partout.

## User story

En tant qu'utilisateur, je veux préremplir un banc à partir d'un fichier existant
ou d'un run précédent, afin de relire, adapter et relancer sans repartir de zéro.

## Les parcours

- **`benchy new`** (CLI) importe un banc en réécrivant ses chemins relatifs (`app`,
  `bundle`, `output`…) en absolu par rapport au fichier source :

  ```sh
  ./benchy new --from benches/freedy/bench.yaml            # sur stdout
  ./benchy new --from benches/freedy/bench.yaml --output bench.yaml
  ```

- **Importer une config** (web) : un champ « Importer une config depuis un
  bench.yaml » (avec sélecteur de fichier côté serveur) préremplit tout le
  formulaire depuis un `bench.yaml` existant, chemins réécrits en absolu.
- **Reprendre une config** (web) : depuis un rapport, « Reprendre cette config »
  rouvre le formulaire prérempli. La reprise a trois niveaux de fidélité : le
  `bench.yaml` propre du run (runs lancés depuis l'interface **et** via `benchy
  run`, qui le persiste), sinon le `bench.yaml` ancêtre au-dessus du dossier de
  résultats, sinon une reconstruction au mieux depuis les artefacts.

## Critères d'acceptation

- **Given** un `bench.yaml` avec des chemins relatifs, **When** on l'importe via
  `benchy new` ou `/new?import=`, **Then** ses chemins sont réécrits en absolu et
  le formulaire est prérempli.
- **Given** un run portant son `bench.yaml`, **When** on le reprend, **Then** le
  formulaire reflète exactement ses inputs.
- **Given** un run ancien sans `bench.yaml` propre, **When** on le reprend, **Then**
  la config est retrouvée depuis un `bench.yaml` ancêtre, à défaut reconstruite au
  mieux.
- **Given** un fichier introuvable, un YAML sans mapping racine, ou un banc sans
  configuration, **When** on importe, **Then** la requête est rejetée (400) avec un
  message explicite.
- **Given** `?from=` avec un chemin `..`, **When** on l'ouvre, **Then** il est
  rejeté.

## Cas non triviaux & limites

- Les chemins vides, déjà absolus ou en `~` ne sont pas réécrits.
- L'import préserve commentaires, ordre et valeurs du fichier source.
- Le glisser-déposer depuis un explorateur est géré en meilleur effort (dépend
  d'une URI `file://` fournie par le navigateur).

## Cas de test associés

`internal/server/import_test.go` : `TestNewImportsBenchFileWithAbsolutePaths`,
`TestNewImportErrors`, `TestNewWithoutParamsRendersDefaultForm`,
`TestNewFromInvalidDirIsRejected`, `TestResolveExistingFile`.
`internal/server/server_test.go` : `TestNewPrefillsFormFromRunConfig`,
`TestReuseRecoversBundleFromAncestorBench`.
`internal/spec/import_test.go` : `TestImportAbsolutizesRelativePaths`,
`TestImportPreservesComments`,
`TestImportLeavesEmptyAndConfigsNonSequenceUntouched`, `TestImportErrors`.
