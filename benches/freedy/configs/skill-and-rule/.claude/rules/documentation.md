# Règle — Documentation du projet

Conventions toujours actives pour toute documentation produite sur ce projet
Go. La **démarche** relève de la skill `doc`, qui fournit les templates.

## Objectif — deux exigences non négociables

- **Autonomie de tout lecteur** : quel que soit son profil — développeur, PO,
  expert métier, nouvel arrivant, non-initié ou tout autre — il doit pouvoir se
  servir de la doc seul, sans lire le code ni demander de l'aide. On écrit pour
  ces publics à la fois.
- **Reconstructibilité à l'identique** : la doc doit suffire à reconstruire le
  projet — comportements attendus, **user stories + critères d'acceptation**,
  **cas de test** (nominaux, limites, erreurs), **cas non triviaux + décisions**,
  invariants, commandes de build et configuration. Ce qui n'est ni dans le code
  ni dans la doc est perdu : ce qui n'est pas évident depuis le code **doit**
  être écrit.

## Arborescence & découpe (tient pour un gros projet)

Toute la doc vit à la racine, en Markdown. Un **backbone fixe** + des **pages
par unité** générées dynamiquement, pour que la doc grossisse avec le projet
sans page fourre-tout :

- `README.md` — porte d'entrée : présentation, prérequis, commandes
  build/run/test (`Makefile`), structure, lien vers `docs/`.
- `docs/index.md` — **index par public** : oriente chaque profil vers son
  parcours.
- `docs/architecture/` — le **comment** (dev) : **une page par package**,
  nommée en **miroir exact du chemin** sous l'arbre de code
  (`docs/architecture/internal/renderer.md` pour `internal/renderer`) — il en
  faut une pour **chaque** dossier de `internal/` contenant du `.go`.
  `docs/architecture/index.md` liste les pages ; `docs/architecture/decisions/`
  accueille les ADR.
- `docs/fonctionnel/` — le **quoi** (sans jargon), **découpé par contexte
  métier** (voir DDD ci-dessous) : `docs/fonctionnel/<contexte>/` avec une page
  par fonctionnalité / parcours, plus `docs/fonctionnel/index.md`.
- `docs/domaine/` — le **pourquoi**, **découpé par contexte** : `glossaire.md`
  (index canonique du vocabulaire) et `regles-metier.md` (index des règles à
  identifiant stable `RG-01…`) au niveau racine, puis un dossier
  `docs/domaine/<contexte>/` par contexte métier.
- `docs/traceabilite.md` — la **matrice de traçabilité** (voir Artefacts).

Règles de découpe :

- **Une page = une unité cohérente** (un package, une fonctionnalité, un
  parcours). Jamais de page qui cumule plusieurs sujets ni trop longue pour être
  lue d'un trait : au-delà, scinder et relier.
- **Chaque dossier a un `index.md`** qui situe et liste ses pages → navigation
  qui passe à l'échelle.
- **Anti-duplication** : une information à un seul endroit (source canonique,
  ex. le glossaire), les autres pages y renvoient par lien relatif. Évite la
  dérive et le gonflement.
- **Matrice scindable** : sur un gros projet, une sous-matrice par domaine et
  `docs/traceabilite.md` devient leur index.
- Nommage `kebab-case` ; liens Markdown relatifs entre pages.

## Découpe par contexte métier (DDD)

Le fonctionnel et le domaine sont organisés par **contexte métier borné**
(bounded context), pas en vrac : chaque contexte est un domaine cohérent avec
son vocabulaire et ses règles propres. Sur freedy, par exemple : **rendu**
(pipeline WebGPU, surface, frames), **texte** (rastérisation du label en
texture), **plateforme** (fenêtre native GLFW vs canvas web, boucle de frames).

- Identifier les contextes depuis le code (packages, responsabilités) et le
  vocabulaire ; en dresser la liste avant d'écrire.
- `docs/domaine/<contexte>/` : une page décrivant le contexte, son **glossaire**
  local et ses **règles** (`RG-<contexte>-01…`). Les fichiers racine
  `glossaire.md` / `regles-metier.md` en sont l'**index** (renvois par lien).
- `docs/fonctionnel/<contexte>/` : les fonctionnalités du contexte.
- Un terme/règle appartient à **un seul** contexte (source canonique) ; les
  autres y renvoient. Les termes transverses vont dans le glossaire racine.

## Contraintes vérifiées automatiquement (à respecter à la lettre)

Des checks déterministes valident la doc — les satisfaire n'est pas optionnel :

- **Une page d'architecture par package `internal/`** : pour chaque dossier de
  `internal/` contenant du `.go`, un `docs/architecture/<chemin>.md` (miroir
  exact du chemin, ex. `internal/label` → `docs/architecture/internal/label.md`).
- **Matrice ancrée dans de vrais tests** : `docs/traceabilite.md` cite les
  **noms exacts** des fonctions `func Test…` présentes dans les `*_test.go`
  (copiés tels quels, sans reformuler).
- **Zéro placeholder** : aucun `TODO` ni gabarit non rempli ne subsiste dans
  `docs/` — relire et compléter chaque emplacement du template.

## Artefacts obligatoires

- **Matrice de traçabilité** : tableau feature → user story → critères
  d'acceptation → cas de test (`*_test.go`) → code. C'est le cœur de la
  reconstructibilité ; elle cite de vrais noms de tests.
- **ADR** sous `docs/architecture/decisions/`, un fichier numéroté par décision
  non triviale (contexte / décision / conséquences) — fige le *pourquoi* des
  cas pièges (états de surface, `RowsPerImage`, `LockOSThread`…).
- **Diagrammes Mermaid** (texte, pas de binaire) pour les flux et la couture
  plateforme/rendu.
- **User stories en Gherkin** (Given/When/Then) reliées aux tests.

## Rédaction

- **En français** ; code et identifiants en anglais, cités tels quels
  (`renderer.New`, `main_web.go`…). Fonctionnel et domaine **sans jargon**.
- Chaque page : titre `H1`, une phrase d'intro, frontmatter minimal
  (`titre`, `public`, `sources:` chemins de code couverts).
- **Fidélité au code, zéro invention** : on ne documente que ce qui existe ;
  pas de doc de code mort ; en cas de doute, lire le code. `CLAUDE.md` et
  `README.md` font foi.

## Interdits

- **Ne jamais modifier le code, les tests ni la configuration** : la génération
  de doc ne touche qu'à `README.md` et à `docs/`.
- Aucun placeholder résiduel (`TODO`, gabarit non rempli) dans `docs/`.
- Pas de HTML dans le Markdown ; pas de binaire ; pas de secret ni de donnée
  personnelle recopiés.
