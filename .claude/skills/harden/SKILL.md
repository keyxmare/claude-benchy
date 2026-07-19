---
name: harden
description: Durcit le changement en cours — tests d'abord, nettoyage sous filet, audits parallèles (standards, perf, a11y + natifs correctness/sécurité), corrections, convergence, gates complets. À invoquer après un développement — "durcis", "blinde", "passe les audits", "/harden". Un état des lieux sans modification relève de /audit.
argument-hint: [périmètre — défaut : feature en cours ; sinon depuis <ref>, dossier]
allowed-tools: Task, Skill, Read, Grep, Glob, Edit, Write, Bash(git:*), Bash(make:*), Bash(docker compose:*)
---

# harden — durcir le changement en cours

Sécuriser par les tests, nettoyer sous filet, auditer en parallèle,
converger, prouver. Périmètre : la feature en cours (commits de branche +
working tree), ou celui passé en argument. Périmètre vide → le dire et
proposer de préciser une plage.

Chaque étape vérifie un état final ; déjà satisfait (ex. red-green déjà
déroulé par `fix`, nettoyage déjà fait) → le constater et passer à la
suivante.

## 1. Tests d'abord

Invoquer `test` : couvrir le changement (red-green pour un correctif).
Déjà couvert → vérifier la suite ciblée verte et passer. Rien ne se nettoie
sans filet.

## 2. Nettoyage sous filet

Nettoyer le diff sous filet de tests : code mort, simplifications,
réutilisations manquées — puis relancer la suite ciblée. Suite rouge après
nettoyage → annuler le nettoyage, ne pas corriger en avant.

## 3. Audits en parallèle

Lancer en un seul message (parallèle), read-only :

- agents : `standards-auditor` et `perf-auditor` ; `a11y-i18n-auditor` si du
  front est touché, `rgpd-auditor` si le périmètre touche aux données
  personnelles (entités, logs, exports, formulaires), `doc-sync-auditor` si
  le projet a un dossier docs/ ;
- natifs s'ils sont disponibles : `code-review` (correctness) et
  `security-review`.

Chaque agent reçoit le périmètre explicitement dans son prompt — il
remplace leur résolution git autonome.

## 4. Corrections

Trier les findings : bloquant/majeur → corriger maintenant ; mineur →
corriger si trivial, sinon le lister en sortie avec sa justification. Un
finding s'écarte par une règle du référentiel chargé, pas par convenance.

## 5. Convergence

Corrections non triviales sur du code de prod → relancer les audits
concernés en passant le périmètre des corrections et la liste des findings
déjà justifiés à ne pas re-signaler. Au-delà de deux cycles, s'arrêter et
lister les findings restants.

## 6. Doc & gates

- Le changement modifie la façon de lancer, tester ou déployer → README mis
  à jour.
- Doc vivante (`.claude/rules/documentation.md`) : pages dont le `sources`
  intersecte le périmètre → alignées sur le code ; comportement ou module
  nouveau sans page → la créer (page architecture en miroir du chemin,
  matrice de traçabilité et index tenus à jour).
- Invoquer `check` (suite complète). Gate rouge → corriger et relancer ;
  toujours rouge après deux passes → s'arrêter et remonter.

## Sortie

Tests ajoutés et verts, nettoyage effectué, findings corrigés ou justifiés
(bloquants restants mis en évidence), gates verts. Le commit relève de
`commit`.
