---
name: commit
description: Crée un ou plusieurs commits Conventional Commits à partir des changements en cours — un sujet par commit, contrôle secrets/debug avant stage. À invoquer pour "commit", "committe", "/commit".
allowed-tools: Read, Grep, Glob, Bash(git:*), Bash(make:*)
---

# commit — Conventional Commits

## 1. État des lieux

`git status` (untracked compris) + `git diff` + `git diff --staged`.

- Rien à committer → le dire et s'arrêter.
- Identifier les sujets distincts : un sujet = un commit ; plusieurs sujets
  → staging sélectif (jamais de `git add -A` par réflexe) et commits
  successifs.
- Untracked manifestement liés au sujet → les stager ; douteux (artefacts,
  fichiers locaux) → demander.

## 2. Contrôles avant stage

- Aucun secret dans le diff (clé, token, mot de passe, URL avec
  identifiants) — au moindre doute, stop et demander.
- Aucune trace de debug (`var_dump`, `dd()`, `console.log`…).
- Branche : vérifier la branche courante contre la politique du contexte
  chargé ; si elle l'interdit, créer la branche d'abord
  (`git switch -c <type>/<sujet>`).

## 3. Gates

Le hook `commit-gate` exécute `make check-fast` (à défaut `check`) au moment
du commit — ne pas le doubler en le lançant soi-même avant. Gate rouge →
corriger puis recommitter, jamais de `--no-verify`.

## 4. Message

Format Conventional Commits du référentiel chargé ; le corps optionnel
explique le pourquoi. Exemple : `fix(cart): keep voucher applied after login`.

## 5. Vérification

`git log --oneline` + `git status` : arbre propre, un sujet et un message
conforme par commit.
