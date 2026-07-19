---
titre: Bundle de config et config de base
public: tous
sources:
  - internal/workspace/workspace.go
  - internal/spec/spec.go
---

# Bundle de config et config de base

> Le bundle est le dossier de configuration superposé à l'app : c'est ce qu'on
> teste. L'option `keepBaseConfig` décide s'il remplace ou complète la config de
> l'app.

## User story

En tant qu'auteur de configuration, je veux superposer mes fichiers de config sur
l'app testée et choisir s'ils remplacent ou complètent ceux de l'app, afin de
comparer soit ma surcouche seule, soit son effet cumulé avec la base du projet.

## Ce qu'est un bundle

Un dossier superposé tel quel sur la copie de l'app. On y place ce qu'on veut
tester au niveau projet :

```text
configs/strict-rules/
├── CLAUDE.md                  # instructions projet
└── .claude/
    ├── rules/style.md         # règles référencées
    └── skills/<nom>/SKILL.md  # skills projet à évaluer
```

## Remplacer ou compléter

- Par défaut (`keepBaseConfig: false`), un fichier du bundle **remplace** celui de
  l'app à même chemin : le `CLAUDE.md` du bundle écrase celui de l'app.
- Avec `keepBaseConfig: true`, le `CLAUDE.md` de l'app est **conservé** et celui du
  bundle lui est **ajouté à la suite** (séparés par une ligne vide).
- Les fichiers à chemins distincts (`.claude/rules/*`, `.claude/skills/*`)
  s'ajoutent dans tous les cas.

Voir [ADR-0002](../../architecture/decisions/0002-keepbaseconfig-remplacer-vs-fusionner.md)
et la règle [RG-banc-05](../../domaine/banc/index.md#règles).

## Critères d'acceptation

- **Given** `keepBaseConfig: false` et un `CLAUDE.md` dans l'app et le bundle,
  **When** on prépare le workspace, **Then** le `CLAUDE.md` du bundle remplace
  celui de l'app.
- **Given** `keepBaseConfig: true` et un `CLAUDE.md` dans les deux, **When** on
  prépare, **Then** le résultat est « base + ligne vide + surcouche ».
- **Given** `keepBaseConfig: true` mais aucun `CLAUDE.md` dans l'app, **When** on
  prépare, **Then** le `CLAUDE.md` du bundle est utilisé tel quel.
- **Given** un fichier de bundle à chemin distinct, **When** on prépare, **Then**
  il est ajouté quel que soit `keepBaseConfig`.

## Cas non triviaux & limites

- La fusion ne concerne **que** le `CLAUDE.md` en collision ; tout autre fichier du
  bundle écrase.
- Dans l'interface web, l'option est la case « Conserver la config de base de
  l'app » ; le bouton 👁 permet d'inspecter le contenu de l'app et des bundles
  (voir [Explorer les fichiers](../pilotage/explorer-les-fichiers.md)).

## Cas de test associés

`internal/workspace/workspace_test.go` : `TestPrepareOverlaysAndCommits`,
`TestPrepareMergesClaudeMdWhenAsked`, `TestPrepareMergeCreatesClaudeMdWhenAppHasNone`,
`TestPrepareGitAppUsesCommittedTree`.
Côté formulaire web : `TestFormCarriesKeepBaseConfig` (`internal/server/form_test.go`).
