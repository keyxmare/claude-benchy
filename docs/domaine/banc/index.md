# Contexte métier — banc

> *Quoi* comparer. Vocabulaire et règles pour définir un banc d'essai.

## Glossaire local

- **Banc** — une expérience de comparaison : un prompt, une app de test, et
  plusieurs configs à mettre en concurrence.
- **Fichier de banc (`bench.yaml`)** — la description YAML du banc. Ses champs
  sont documentés dans [définir un banc](../../fonctionnel/banc/definir-un-banc.md).
- **App de test (`app`)** — le dossier de l'application sur laquelle Claude
  travaille. Le dépôt git n'est pas requis ; s'il en est un, seul l'arbre `HEAD`
  est utilisé.
- **Config (configuration)** — une variante comparée : un `name` unique, un
  `bundle`, et d'éventuels *overrides* `model`/`prompt`.
- **Bundle** — un dossier superposé tel quel sur la copie de l'app : c'est ce
  qu'on teste au niveau projet (`CLAUDE.md`, `.claude/rules/*`,
  `.claude/skills/*`…).
- **Prompt** — la tâche confiée à Claude, en clair (`prompt`) ou dans un fichier
  (`promptFile`).
- **Modèle (`model`)** — le modèle Claude utilisé (défaut `sonnet`),
  surchargeable par config.
- **`keepBaseConfig`** — bascule décidant si le `CLAUDE.md` du bundle **remplace**
  celui de l'app ou lui est **ajouté** (voir
  [ADR-0002](../../architecture/decisions/0002-keepbaseconfig-remplacer-vs-fusionner.md)).
- **`runs`** — nombre de répétitions par config (mesure de la variance).
- **`concurrency`** — nombre de runs Docker menés en parallèle.
- **`retries`** — nombre de relances d'un [run sans effet](../execution/index.md).

## Règles

- **RG-banc-01 — Validité d'un banc.** Un banc est invalide sans `app` pointant un
  dossier existant, sans au moins une config, ou avec une config sans nom unique,
  sans bundle existant, ou sans prompt résoluble.
  Source : `internal/spec/spec.go` (`validate`). Tests :
  `TestLoadErrors`, `TestLoadMissingBundle` (`internal/spec/spec_test.go`).
- **RG-banc-02 — Prompt exclusif et hérité.** `prompt` et `promptFile` sont
  mutuellement exclusifs à tout niveau ; une config sans prompt hérite du prompt
  de tête. Source : `internal/spec/spec.go` (`resolvedPrompt`, `resolve`). Tests :
  `TestLoadPromptFile`, `TestLoadErrors`, `TestLoadDefaultsAndOverrides`.
- **RG-banc-03 — Sémantique de `retries`.** Non renseigné → défaut 2 ; `0`
  explicite → désactivé ; négatif → invalide. Source : `internal/spec/spec.go`
  (`RetryCount`, `validate`). Tests : `TestLoadRetries`, `TestLoadErrors`.
- **RG-banc-04 — Champs inconnus rejetés.** Un champ YAML inconnu rend le banc
  invalide (`KnownFields(true)`). Source : `internal/spec/spec.go` (`Load`).
  Tests : `TestLoadErrors`.
- **RG-banc-05 — Fusion ciblée du `CLAUDE.md`.** `keepBaseConfig` ne fusionne que
  le `CLAUDE.md` en collision ; les autres fichiers du bundle écrasent toujours.
  Source : `internal/workspace/workspace.go` (`copyTree`, `appendFile`). Tests :
  `TestPrepareOverlaysAndCommits`, `TestPrepareMergesClaudeMdWhenAsked`,
  `TestPrepareMergeCreatesClaudeMdWhenAppHasNone` (`internal/workspace/workspace_test.go`).

## Fonctionnalités rattachées

- [Définir un banc](../../fonctionnel/banc/definir-un-banc.md)
- [Bundle de config et config de base](../../fonctionnel/banc/bundle-et-config-de-base.md)
