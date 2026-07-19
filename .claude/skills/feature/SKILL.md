---
name: feature
description: Fil conducteur d'une feature de bout en bout — branche selon la politique du contexte, conception en plan mode, développement, durcissement (/harden), validation utilisateur, commit et livraison. À invoquer pour "développe", "implémente", "ajoute la feature", "/feature". Corriger un bug relève de /fix.
argument-hint: <description de la feature>
allowed-tools: Task, Skill, Read, Grep, Glob, Edit, Write, Bash(git:*), Bash(make:*), Bash(docker compose:*)
---

# feature — de l'idée à la livraison

## 0. Cadrage

Reformuler le besoin en une phrase. Changement trivial (quelques lignes,
localisé, couvert par les tests existants) → sauter `clarify` et la
conception (§2), alléger le durcissement (§4) : `test`, `standards-auditor` et
`check` — l'annoncer explicitement. En cas de doute sur la trivialité, tout
dérouler. Sinon, invoquer `clarify` pour lever les ambiguïtés métier et mettre
à jour la doc de domaine ; ses critères d'acceptation cadrent la conception.

## 1. Branche — on travaille sur `main`

Ce projet commit directement sur `main` : pas de branche ni de PR.

## 2. Conception

Sujet non trivial → passer en plan mode : repartir du cadrage `clarify`,
explorer le code existant (déléguer les recherches larges à un agent
Explore), proposer une approche argumentée ; la validation utilisateur fait
partie du plan mode. Plan refusé → le retravailler, jamais d'implémentation
sans validation.

## 3. Développement

Implémenter le plan validé étape par étape — cohérence locale, référentiel
chargé. Bloqué ou approche remise en cause → s'arrêter et remonter, jamais
de contournement silencieux.

## 4. Durcissement

Invoquer `harden` (tests, nettoyage, audits parallèles, convergence,
gates). Findings bloquants non résolus en sortie → pas de livraison,
remonter.

## 5. Livraison

Présenter l'auto-review — diff complet de la feature, résultat des tests,
findings corrigés ou justifiés — et **attendre la validation explicite de
l'utilisateur**. Puis invoquer `commit` et pousser sur `main` (`git push`).

## Sortie

Feature livrée selon la politique du contexte, gates verts, auto-review
validée par l'utilisateur.
