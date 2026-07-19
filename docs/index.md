# Documentation — claude-benchy

> Point d'entrée unique de la documentation. Choisissez votre parcours.

`claude-benchy` (« benchy ») est un **banc d'essai** : il compare plusieurs
configurations de Claude Code (skills, rules, `CLAUDE.md`) sur un même prompt et
une même application de test, de façon isolée et reproductible. Pour chaque
configuration, benchy prépare une copie jetable de l'app, y superpose la
configuration, lance Claude sans interaction dans un conteneur Docker, puis
capture le diff, le transcript et les métriques, et produit un rapport comparatif.

## Par où commencer, selon votre profil

- **Nouvel arrivant / non-initié** — benchy sert à répondre à la question
  « quelle configuration de mon assistant produit le meilleur résultat ? » en la
  mesurant plutôt qu'en la devinant. Commencez par
  [le fonctionnel](fonctionnel/index.md).
- **Du métier / PO** — [Fonctionnalités & parcours](fonctionnel/index.md) et
  [Glossaire](domaine/glossaire.md).
- **Développeur** — [Architecture](architecture/index.md) (packages, flux du
  pipeline, build), puis les [décisions (ADR)](architecture/decisions/index.md).
- **Reconstruire le projet** — [Matrice de traçabilité](traceabilite.md)
  (fonctionnalité → user story → tests → code).

## Sommaire

- [Architecture](architecture/index.md) · [Décisions (ADR)](architecture/decisions/index.md)
- [Fonctionnel](fonctionnel/index.md)
- [Glossaire](domaine/glossaire.md) · [Règles métier](domaine/regles-metier.md)
- [Traçabilité](traceabilite.md)

## Vocabulaire minimal

Pour lire le reste sans blocage : un **banc** est décrit par un **fichier de
banc** (YAML) ; il compare des **configs**, chacune apportant un **bundle** (un
dossier de configuration superposé à l'app) ; chaque config est exécutée un ou
plusieurs **runs** ; l'**évaluation** (optionnelle) note les productions. Toutes
ces notions sont définies dans le [glossaire](domaine/glossaire.md).
