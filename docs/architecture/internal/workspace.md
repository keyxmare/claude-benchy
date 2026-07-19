---
titre: internal/workspace — copie isolée et baseline git
public: dev
sources:
  - internal/workspace/workspace.go
---

# internal/workspace — copie isolée et baseline git

> Prépare une copie jetable de l'app de test, y superpose le bundle de config,
> et commit une baseline git — pour qu'un diff ultérieur ne montre que ce que
> Claude a changé.

## Rôle & responsabilités

L'unique fonction exportée est `Prepare(app, bundle, dst string, mergeClaudeMd
bool) error`. Elle :

- copie l'app dans `dst` (arbre `HEAD` si l'app est un dépôt git, sinon copie
  verbatim) ;
- superpose les fichiers du bundle par-dessus ;
- commit un commit `baseline` git (auteur/committer `benchy`).

Elle **ne** lance pas Docker, ne calcule pas de diff (c'est `internal/diffcap`),
ne choisit pas d'image et ne lit pas le banc. Elle ignore le nom
`keepBaseConfig` : elle reçoit un simple booléen `mergeClaudeMd`.

## Flux principaux

**Copie de l'app** : `isGitRepo(app)` est vrai si `app/.git` est un dossier. Si
dépôt git, `gitArchive` pipe `git archive --format=tar HEAD` dans `tar -x` : seul
l'arbre HEAD committé entre dans le sandbox (ignorés, non suivis et artefacts de
build restent dehors). Sinon, copie récursive verbatim, `.git` de tête exclu.

**Superposition du bundle** : parcours de `bundle` ; le `.git` éventuel est
ignoré ; dossiers recréés, liens symboliques recréés, fichiers copiés — **sauf**
quand `mergeClaudeMd && nom == "CLAUDE.md" && cible existe`, auquel cas le contenu
du bundle est **ajouté à la suite** de celui de l'app (séparés par `"\n\n"`).

**Baseline git** : dans `dst`, `git init -q`, `git add -A`, `git commit -q
--no-gpg-sign -m baseline`, avec identité fixe `benchy`. Chaque appel git préfixe
`-c safe.directory=*` (nécessaire pour git sur un montage bind en conteneur).

## Dépendances & cibles

Bibliothèque standard uniquement, plus les binaires `git` et `tar` disponibles
dans le sandbox. Appelé par `internal/runner` (`Prepare(s.App, bundle,
workspaceDir, s.KeepBaseConfig)`).

## Invariants & pièges

- **La baseline inclut le bundle** : la superposition a lieu *avant* le commit, de
  sorte qu'un diff ultérieur isole les seules modifications de Claude — voir
  [ADR-0009](../decisions/0009-baseline-git-inclut-le-bundle.md).
- **Reproductibilité** : pour une app git, seul l'arbre HEAD entre dans le
  sandbox.
- **Arbre propre après baseline** (invariant : `git status --porcelain` vide).
- **Identité de commit déterministe** (`benchy`, `--no-gpg-sign`).
- **`keepBaseConfig`** ne concerne que la collision du `CLAUDE.md` ; les autres
  fichiers du bundle écrasent toujours — voir
  [ADR-0002](../decisions/0002-keepbaseconfig-remplacer-vs-fusionner.md). Si l'app
  n'a pas de `CLAUDE.md`, celui du bundle est utilisé tel quel même avec la fusion
  active.

## Cas de test associés

`workspace_test.go` : `TestPrepareOverlaysAndCommits`,
`TestPrepareGitAppUsesCommittedTree`, `TestPrepareMergesClaudeMdWhenAsked`,
`TestPrepareMergeCreatesClaudeMdWhenAppHasNone`.
