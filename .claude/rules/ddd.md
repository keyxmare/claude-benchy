---
paths:
  - "**/*.go"
---

# Règle — Domain-Driven Design (Go)

Positionnement du projet sur le DDD **stratégique** (contextes bornés, context
map, langage omniprésent) et **tactique** (agrégats, value objects, entités,
repositories, services de domaine, événements de domaine, couche
anti-corruption). Cette règle porte le **jugement** de modélisation ;
`standards-auditor` la confronte au diff. Elle **complète** `go-design.md`, ne la
remplace pas, et lui reste **subordonnée**.

## Arbitrage — le DDD sert la simplicité, il ne l'écrase pas

L'ordre de `go-design.md` (§ Arbitrage) prime **intégralement** :

1. **KISS / YAGNI d'abord.** Un pattern tactique (agrégat, repository, service,
   événement) **se mérite** exactement comme OCP : on ne l'introduit qu'à
   variation ou besoin **avérés**, jamais par ce que « le DDD prescrit ». Un
   pipeline linéaire n'a pas à devenir une soupe d'abstractions pour cocher des
   cases.
2. **Le stratégique est peu coûteux, le tactique se paie.** Nommer les contextes,
   tenir le langage omniprésent et isoler le cœur de l'infra **ne coûtent
   presque rien** et rapportent tout de suite : ce sont les attendus par défaut.
   Agrégats, repositories et événements **cérémoniels** ne s'ajoutent que
   lorsqu'ils suppriment une complexité réelle, pas quand ils en ajoutent.
3. **En cas de doute, le modèle le plus simple à supprimer gagne** — comme dans
   `go-design.md`.

Un écart à cette règle qui contredirait KISS/YAGNI n'est **pas** un écart à
corriger : c'est cette règle qui cède.

## Stratégique — contextes bornés & context map

- **Le package `internal/*` est le contexte borné.** Chaque package possède son
  modèle, son langage et sa frontière ; on ne fait pas fuiter les types d'un
  contexte dans un autre sans traduction. La couture du pipeline est la context
  map :

  ```text
  spec ─▶ workspace ─▶ docker ─▶ claude ─▶ diffcap ─▶ runner ─▶ report
                                                         │
                                        (orchestre tout ▲┘, agrège dans report)
  ```

  - **Bench Specification** (`spec`) — amont : charge et **valide** le banc
    (`Spec`, `Config`, `Evaluate`, `Check`), résout les chemins et les creds.
    Fournit un modèle valide au reste ; personne ne re-valide en aval.
  - **Workspace Provisioning** (`workspace`) — matérialise une copie isolée +
    baseline git par config.
  - **Sandbox Execution** (`docker`) — exécute le conteneur ; le monde extérieur
    (daemon Docker) est **derrière l'interface `Runner`**.
  - **Transcript** (`claude`) — **couche anti-corruption** : traduit le
    `stream-json` externe de Claude en modèle interne (`Metrics`, `Usage`) et en
    lignes lisibles. Le format externe ne remonte jamais tel quel.
  - **Diff Capture** (`diffcap`) — capture le changement en `Result` / `Stats`.
  - **Orchestration** (`runner`) — contexte **cœur** (voir plus bas) ; consomme
    tous les autres, n'est consommé par aucun.
  - **Reporting** (`report`) — **noyau partagé** (shared kernel) : les types de
    résultat (`Report`, `RunReport`, `ConfigEval`, `ConfigScore`) sont le
    langage commun de `runner`, `cmd/benchy` et `internal/server`. Un changement
    y est un changement de contrat partagé, à traiter comme tel.

- **`cmd/benchy` et `internal/server` sont la racine de composition**, pas des
  contextes de domaine : ils câblent, ils ne décident pas.

## Langage omniprésent — un concept, un nom, partout

- Le lexique du banc est **unique et fait foi** dans le code, la doc
  (`docs/domaine/`) et la CLI : `Bench`, `Config`, `Job` (= config × run),
  `Run` / `RunReport`, `Baseline`, `Diff`, `Judge` / `Evaluation`, `Rubric`,
  `Check`, `Score`. Pas de synonyme qui dérive (`result` vs `report` vs
  `outcome` pour la même chose).
- Le concept métier central — un **run sans effet** (aucun outil appelé ou diff
  vide) écarté des agrégations et relancé — porte **un** nom explicite dans le
  code, pas une condition anonyme recopiée. C'est l'invariant à préserver du
  `CLAUDE.md` : il mérite d'être nommé, pas deviné.
- Renforce `SSOT` et `PoLA` de `go-design.md` : le nom du domaine est la source
  unique, le comportement ne surprend pas.

## Tactique — modéliser, sans cérémonie

- **Value objects contre l'obsession du primitif.** Une donnée porteuse
  d'invariant ne reste pas un `string`/`int` nu qu'on revalide partout : un type
  qui **valide à la construction** (Fail Fast) et devient impossible à mal
  former. Cibles naturelles : `Score` (borné), `Concurrency` / `RetryCount`
  (> 0), le `Level` normalisé (`report.NormalizeLevel`), un chemin résolu
  relatif au banc. Immuables, comparés par valeur, sans identité.
- **Entités vs agrégats.** Ce qui a une **identité et un cycle de vie** (un
  `RunReport`, un `Job`) est une entité ; ce qui n'est qu'une valeur
  (`ConfigScore`, `Stats`, `CriterionEval`) est un value object. Un **agrégat**
  ne se déclare que si un invariant doit tenir sur un groupe : alors l'accès
  passe par sa **racine**, qui garde la cohérence — pas de mutation d'un membre
  interne par-derrière. `Bench` (spec + configs) et `Report` (runs + agrégats)
  sont les racines candidates ; ne les fermer que si un invariant l'exige, sinon
  YAGNI (§ Arbitrage).
- **Modèle riche, pas anémique.** La logique de domaine vit **sur** le type qui
  détient la donnée, pas dans une fonction utilitaire qui l'extrait pour décider
  à sa place (`Tell, Don't Ask` de `go-design.md`). Un `RunReport` sait dire
  s'il est sans effet ; un `Report` sait se classer.
- **Services de domaine** pour la logique qui n'appartient **à aucune** entité :
  détection de run sans effet et **politique de relance** (`runner`), **scoring**
  (`report.ScoreFromCriteria`, agrégation `ConfigScore`), **classement**
  (`rankByScore`). Sans état, nommés par l'action du domaine.
- **Repositories** pour abstraire la **persistance** (arbre d'artefacts sur le
  FS : `diff.patch`, transcript, `metrics.json`, rapports) derrière une interface
  de collection **côté consommateur** (ISP/DIP) — *si et seulement si* la
  variation de stockage est avérée. Aujourd'hui elle ne l'est pas : ne pas
  introduire de repository spéculatif (§ Arbitrage). C'est le point le plus
  exposé à la sur-ingénierie.
- **Événements de domaine.** Le flux de progression existant (`AgentEvent`,
  callbacks `Options.agent` / `Options.log`) **est** déjà un flux d'événements
  de domaine : le cœur les émet, l'infra les consomme (sortie live CLI, SSE du
  dashboard). Les traiter comme tels — noms au **passé** de fait métier
  (`RunCompleted`, `RunHadNoEffect`), le producteur ignore le consommateur.
  Ne pas en inventer d'autres sans consommateur réel.

## Cœur vs infrastructure — la frontière DDD

- Le **cœur** (décisions : qu'est-ce qu'un run sans effet, faut-il relancer,
  quel score, quel classement) **ne dépend pas** de l'infrastructure (Docker,
  système de fichiers, réseau, horloge, CLI `claude`). L'infra est **derrière
  des interfaces étroites** définies côté cœur (`docker.Runner` en est le
  modèle) et **implémentée** en périphérie. C'est le `DIP`/`SoC`/`OP` de
  `go-design.md`, énoncé du point de vue du domaine.
- La couche **anti-corruption** (`claude` parsant le `stream-json`) protège le
  modèle interne des formats externes : leur schéma peut changer, le cœur non.

## Où chaque décision se joue

- **Conception / plan** (le moins cher) : frontières de contexte, langage
  omniprésent, quelles données sont des value objects, où passe la frontière
  cœur/infra. Décidés **avant** d'écrire.
- **Écriture** : modèle riche (TDA), invariants au constructeur (Fail Fast),
  pas de type externe qui fuit dans le cœur.
- **Revue / durcissement** : agrégats, repositories et événements sont
  reconfrontés au filtre « se méritent-ils ? » (§ Arbitrage). `standards-auditor`
  cite cette règle **et** `go-design.md` à l'appui ; en conflit, `go-design.md`
  tranche.
