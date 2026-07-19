---
paths:
  - "**/*.go"
---

# Règle — Principes de conception (Go)

Positionnement du projet sur SOLID, les fondamentaux, la discipline et le
pragmatisme, et **ordre d'arbitrage** quand ils s'opposent. Le plancher
**mesurable** (complexité, duplication, taille d'interface, erreurs) est tenu
par les linters de `.golangci.yml` (gate `make check`) ; cette règle porte le
**jugement**, qu'aucun linter n'attrape, et que `standards-auditor` confronte au
diff. Elle ne remplace pas les linters, elle les complète.

## Arbitrage — ces principes se contredisent, ne les applique pas tous à fond

L'objectif n'est pas de maximiser chaque principe, c'est de **choisir**. Ordre
de priorité par défaut, du plus fort au plus faible :

1. **KISS / YAGNI d'abord.** La solution la plus simple qui répond au besoin
   *actuel* gagne. Pas d'abstraction, de généricité ni de point d'extension pour
   un besoin hypothétique.
2. **Rule of Three garde-fou de DRY.** On ne factorise qu'à la **troisième**
   occurrence réelle. Deux passages qui se ressemblent par coïncidence (pas par
   nature) restent dupliqués. `dupl` signale la duplication *mécanique* ; c'est à
   toi de juger si elle est *essentielle* (même raison de changer) ou
   *accidentelle*.
3. **OCP se mérite.** On n'ouvre à l'extension (interface, stratégie, hook)
   qu'une fois la variation **avérée**, pas anticipée — sinon YAGNI l'emporte.
4. En cas de doute, **le code le plus simple à supprimer** gagne sur le code le
   plus « propre ».

## SOLID — appliqué au style Go

- **SRP** : un type / une fonction a **une raison de changer**. Symptômes à
  corriger : fonction qui dépasse le seuil `funlen`, fichier qui cumule rendu +
  I/O + parsing (cf. le découpage `internal/*` du projet). Le seuil linter est
  un signal, pas la définition — une fonction courte peut violer SRP.
- **OCP** : cf. arbitrage §3. En Go, extension = nouvelle implémentation d'une
  interface **étroite**, pas `if/switch` sur un type qui enfle à chaque cas.
- **LSP** : toute implémentation d'une interface respecte son contrat (erreurs,
  nil, effets de bord) — une implémentation qui panique là où les autres
  renvoient une erreur casse la substituabilité. Non mesurable : c'est un point
  de revue.
- **ISP** : interfaces **minimales**, définies **côté consommateur**. `interface`
  d'un seul verbe est la norme Go (`io.Reader`). `interfacebloat` borne la
  taille ; l'esprit va plus loin — ne demande que ce que tu utilises.
- **DIP** : le métier dépend d'**abstractions**, l'infra les implémente. Idiome
  Go : **accepter des interfaces, renvoyer des structs**. Les ressources
  externes du projet (Docker, réseau, horloge, CLI `claude`) sont derrière des
  interfaces (`docker.Runner`) — le cœur ne les touche jamais en direct.

## Fondamentaux

- **DRY** : une connaissance = une représentation. Soumis à Rule of Three
  (§2). DRY vise le savoir dupliqué, pas les lignes qui se ressemblent.
- **KISS** : cf. §1. Complexité bornée par `gocyclo` / `gocognit` / `nestif`.
- **YAGNI** : cf. §1. Pas de paramètre, champ, branche ou export « au cas où » —
  `unparam` et `unused` en attrapent une partie.
- **SoC** : séparer les préoccupations par package / type (le pipeline
  `spec → workspace → docker → claude → diffcap → runner → report` en est
  l'ossature). Ne pas mêler décision métier et pilotage d'I/O dans la même
  fonction.
- **SSOT** : une donnée a une source unique. Pas de constante recopiée, pas
  d'état dérivé stocké en double — on le calcule.

## Discipline

- **LoD (Loi de Déméter)** : ne parle qu'à tes voisins directs, pas de
  `a.B().C().D()`. Expose une méthode qui fait le travail plutôt que la chaîne.
- **TDA (Tell, Don't Ask)** : dis à l'objet quoi faire, ne lui extrais pas son
  état pour décider à sa place. Un getter suivi d'un `if` sur le résultat est un
  signal.
- **CQS (Command Query Separation)** : une méthode **change l'état** (command,
  renvoie peu ou rien) **ou renvoie une valeur** (query, sans effet de bord),
  jamais les deux. Exception idiomatique Go tolérée : `val, ok :=` et `val, err
  :=`.
- **CoI (Composition over Inheritance)** : Go n'a pas d'héritage — composer par
  embedding et interfaces. Ne pas simuler une hiérarchie de types.
- **PoLP (Least Privilege)** : surface exportée **minimale** (non-exporté par
  défaut, on exporte au besoin) ; portée la plus étroite ; permissions et accès
  au strict nécessaire. `revive` / `unused` aident sur l'export mort.

## Pragmatisme

- **FF (Fail Fast)** : valider tôt, échouer clair. Erreurs vérifiées et
  enveloppées avec contexte (`errcheck` / `errorlint`), pas ignorées ni avalées.
- **BSR (Boy Scout Rule)** : laisser le code touché plus propre qu'à l'arrivée —
  nettoyage **borné au périmètre**, sans refactor opportuniste non demandé.
- **RoT (Rule of Three)** : cf. §2, garde-fou de DRY.
- **PoLA (Least Astonishment)** : nommage et comportement sans surprise ; une
  fonction fait ce que son nom dit, rien de plus. Pas de masquage de builtin
  (`min`, `max`…), pas d'effet de bord caché.
- **OP (Orthogonality)** : modules indépendants — un changement ici ne casse pas
  là. Faible couplage, forte cohésion ; c'est ce que teste l'isolation des
  ressources externes derrière interfaces.

## Où chaque principe se joue

- **Conception / plan** (le moins cher) : SRP, OCP, ISP, DIP, SoC, CoI, YAGNI,
  OP — décidés **avant** d'écrire le code.
- **Écriture** : KISS, DRY, LoD, TDA, CQS, FF, PoLA, PoLP, SSOT.
- **Revue / durcissement** : tout est reconfronté ; BSR et Rule of Three s'y
  vérifient. `standards-auditor` cite cette règle à l'appui.
