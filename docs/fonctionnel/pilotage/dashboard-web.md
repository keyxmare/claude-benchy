---
titre: Dashboard web
public: tous
sources:
  - internal/server/server.go
  - internal/server/jobs.go
  - internal/server/form.go
  - internal/server/history.go
---

# Dashboard web

> Configurer, lancer et parcourir les bancs depuis le navigateur, avec une
> progression live et une sortie par agent.

## User story

En tant qu'utilisateur, je veux piloter mes bancs depuis une interface web plutôt
qu'en éditant un fichier, afin de configurer, lancer et suivre une comparaison
sans quitter le navigateur.

## Lancer le dashboard

```sh
./benchy serve                 # http://127.0.0.1 (port 80)
```

Options : `--addr` (défaut `127.0.0.1:80`), `--root` (dossier scanné pour
l'historique et base des chemins relatifs, défaut `.`), `--image` (image sandbox
par défaut). **Localhost par défaut** : ne pas exposer au réseau (voir
[ADR-0008](../../architecture/decisions/0008-dashboard-stdlib-localhost.md)).

## Ce qu'on peut faire

- **Nouveau bench** : un formulaire couvre tous les inputs du fichier de banc
  (prompt, app, model, runs, configs, rubric, checks…) ; « Lancer » démarre le run
  et **diffuse la progression live** (SSE), puis affiche le lien du rapport.
- **Sortie live par agent** : un panneau par agent (config × run) affiche en temps
  réel la sortie de son Claude — texte, appels d'outils, résultats — rendue depuis
  le flux stream-json. Chaque panneau porte un lien « transcript ↗ » qui rejoue le
  feed depuis `transcript.jsonl` (route `/transcript`), consultable après le run.
- **Historique** : la liste de tous les bancs générés sous la racine, chacun
  ouvrant son rapport HTML re-rendu à la volée.

L'image du banc l'emporte sur l'image par défaut du serveur. Le `bench.yaml`
soumis est persisté dans le dossier de résultats.

## Critères d'acceptation

- **Given** le dashboard lancé, **When** on ouvre `/`, **Then** le formulaire de
  nouveau bench s'affiche.
- **Given** un formulaire valide soumis, **When** on lance, **Then** on est
  redirigé vers la page du run et le flux SSE émet des événements `log`, `agent`,
  puis `done`.
- **Given** un formulaire invalide (ex. app vide), **When** on soumet, **Then** le
  formulaire est réaffiché avec les valeurs saisies et le message d'erreur.
- **Given** un `dir` avec `..`, **When** on demande `/report` ou `/transcript`,
  **Then** la requête est rejetée (400).

## Cas non triviaux & limites

- L'état des jobs en mémoire est perdu au redémarrage ; les artefacts restent
  listables via l'historique et les transcripts rejouables.
- Les lignes de config/check du formulaire vides sont ignorées ; `retries` est
  tri-état (blanc → défaut, `0`/`N` préservés).
- Les templates et le CSS sont embarqués (`go:embed`) : modifier l'UI exige un
  rebuild (`make serve-watch` pour le développement).

## Cas de test associés

`internal/server/server_test.go` : `TestDashboardRenders`,
`TestRunLaunchesJobAndStreamsToCompletion`,
`TestRunRerendersFormOnValidationError`, `TestTranscriptRejectsPathTraversal`,
`TestReportRejectsPathTraversal`.
`internal/server/form_test.go` : `TestFormSpecDropsBlankRowsAndParsesLists`,
`TestFormRetriesRoundTrip`, `TestFormSpecRejectsInvalidNumber`,
`TestChooseImagePrefersBenchImage`, `TestBenchDocMarshalsTidyYAML`.
