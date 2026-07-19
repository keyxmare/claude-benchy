---
name: doc
description: >-
  Génère ou complète la documentation du projet — README d'entrée, arborescence
  docs/ (architecture, fonctionnel, domaine), matrice de traçabilité et ADR — de
  façon exhaustive, découpée pour passer à l'échelle, et fidèle au code. À
  invoquer pour « documente le projet », « génère la doc », « quelles zones ne
  sont pas documentées ? ». N'édite jamais le code.
---

# Génération de documentation

Objectif : une doc dont **tout lecteur** (dev, PO, expert métier, nouvel
arrivant, non-initié…) se sert seul, et qui suffit à **reconstruire le projet à
l'identique**. Conventions dans `.claude/rules/documentation.md` (arborescence,
découpe, artefacts, interdits) — s'y tenir. Les gabarits sont dans
`.claude/skills/doc/templates/` : les **copier puis remplir**, ne pas réinventer
la structure (moins de tokens, sortie reproductible).

## 1. Cartographier (lecture seule, exhaustive)

- **Sources qui font foi d'abord** : `CLAUDE.md` et `README.md` — architecture,
  commandes et pièges y sont déjà décrits. La doc les reflète et les développe.
- **Build/exécution** : `Makefile`, `go.mod`, `compose.yaml`, `deploy/`.
- **Unités de code** : `go list ./...` et l'arbre `cmd/`, `internal/`, `web/`.
  Repérer la couture plateforme/rendu et la séparation par build tag
  `!js`/`js`.
- **Comportement** : les `*_test.go` décrivent les cas attendus.

## 2. Planifier la découpe (avant d'écrire)

Établir la liste des pages, pour que la doc grossisse proprement :

- **Backbone** (toujours) : `docs/index.md`, `docs/architecture/index.md`,
  `docs/fonctionnel/index.md`, `docs/domaine/glossaire.md`,
  `docs/domaine/regles-metier.md`, `docs/traceabilite.md`,
  `docs/architecture/decisions/index.md`.
- **Pages par unité** : une page architecture par **package** (miroir de
  l'arbre), une page fonctionnel par **fonctionnalité / parcours**. Une unité =
  une page ; si une page cumule des sujets ou devient trop longue, la scinder.
- **Anti-duplication** : chaque terme/règle défini une seule fois (glossaire /
  regles-metier), référencé ailleurs par lien.

## 3. Générer, unité par unité

Traiter les unités **indépendamment** (contexte borné, coût maîtrisé) : pour
chaque unité, copier le template adapté et le remplir depuis le code.

- Architecture : template `architecture.md` — rôle, flux (diagramme **Mermaid**
  si utile), dépendances/cibles, invariants & pièges (lier les ADR), tests
  associés.
- Fonctionnel : template `fonctionnel.md` — user story + **critères
  d'acceptation en Gherkin** (Given/When/Then), cas non triviaux, tests
  associés.
- Domaine : template `domaine.md` — glossaire + règles `RG-01…`.
- Décisions : template `adr.md` — un fichier numéroté par cas non trivial
  (états de surface, `RowsPerImage`, `LockOSThread`…).
- Index : chaque dossier reçoit son `index.md` qui liste et situe ses pages.

Puis la **matrice de traçabilité** (template `traceabilite.md`) : feature → US →
critères → cas de test (`*_test.go`) → code. Scinder par domaine si volumineux.

## 4. Passe d'auto-vérification (obligatoire, avant de conclure)

Relire la doc produite **contre le code** et combler les trous :

- [ ] chaque package `internal/…` a sa page architecture ;
- [ ] chaque fonctionnalité a US + critères + cas non triviaux + tests ;
- [ ] chaque `*_test.go` apparaît dans la matrice ;
- [ ] chaque terme métier/technique est au glossaire, chaque règle a un `RG-…` ;
- [ ] chaque dossier a son `index.md` ; les liens relatifs résolvent ;
- [ ] un lecteur non-technicien comprend le projet via `docs/index.md` seul ;
- [ ] **aucun placeholder `TODO` ni gabarit non rempli** ne subsiste ;
- [ ] le code, les tests et la configuration **n'ont pas** été modifiés ;
- [ ] tout est en français, en Markdown.
