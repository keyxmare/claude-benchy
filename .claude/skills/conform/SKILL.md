---
name: conform
description: Met tout le projet en conformité avec une ou plusieurs règles chargées — détecte les écarts sur l'existant (pas seulement le changement en cours), applique les corrections mécanisables sous filet de tests, produit un plan pour le jugement, un commit par règle. À invoquer pour "applique go-assets à tout le projet", "mets le projet en conformité avec ma conf", "/conform [règles]". Auditer sans corriger relève de /audit.
argument-hint: [règles — défaut : toutes celles dont le paths matche ; ex. "go-assets go-design"]
allowed-tools: Task, Skill, Read, Grep, Glob, Edit, Write, Bash(git:*), Bash(make:*), Bash(docker compose:*)
---

# conform — mettre le projet en conformité avec ses règles

Applique des règles à **l'intégralité du projet**, pas au seul changement en
cours (ça, c'est `harden`). Réutilise `standards-auditor` (détection),
`refactor` / `fix` (application sous filet), `test`, `check` et `commit`. Un
état des lieux sans modification relève de `audit`.

## 0. Périmètre & règles

- Règles : celles passées en argument, sinon **toutes** les `rules/*.md` dont
  le frontmatter `paths` matche des fichiers du projet.
- Périmètre : tout le projet par défaut ; un sous-arbre si précisé.
- Lister les règles retenues et le périmètre, puis confirmer avant d'agir.

## 1. Classer les règles (déterminant)

Toutes les règles ne s'appliquent pas de la même façon — classer avant de
toucher au code :

- **Mécanisable** (transformation déterministe, comportement inchangé) :
  ex. go-assets (asset inline → `//go:embed`), zéro-commentaire, builtin
  masqué, format. → §3a.
- **Jugement** (arbitrage nécessaire) : ex. go-design (SRP, KISS, OCP…). Pas
  de réécriture aveugle. → §3b.
- **Couverture** : go-tests (100 %). → §3c.

## 2. Filet de tests

Suite verte d'abord (`check`, ou `test` ciblé). Zone à modifier non couverte →
test de caractérisation **avant** toute transformation (délègue à `test`).
Rien ne se transforme sans filet.

## 3. Application, une règle à la fois

Traiter les règles séquentiellement ; **un commit par règle** (revert facile).

### 3a. Mécanisable

1. Détecter tous les sites : `standards-auditor` sur tout le projet, focalisé
   sur la règle (lui passer la règle et le périmètre dans le prompt).
2. Appliquer la transformation partout, comportement préservé.
3. Tests ciblés verts → `commit` (`refactor(scope): …` ou `chore`). Rouge →
   annuler la transformation, ne pas corriger en avant.

### 3b. Jugement

1. Détecter et **prioriser** les écarts (`standards-auditor`) : bloquant →
   mineur.
2. Présenter un **plan** ; n'appliquer que le validé, via `refactor` (une
   transformation atomique par écart, commit par lot cohérent).
3. Aucun écart validé → le lister en sortie, ne rien réécrire.

### 3c. Couverture

Par package, générer les tests manquants (délègue à `test` / `go-testing`)
jusqu'à 100 % sur le code visé. Volume important → annoncer le découpage, un
commit par package.

## 4. Convergence & gates

Re-auditer les règles traitées ; `check` complet vert. Rouge après deux passes
→ s'arrêter et remonter.

## 5. Sortie

Par règle : écarts trouvés / corrigés / différés (avec justification), commits
produits, statut des gates. Rien de mécanisable ne reste non traité sans raison.
