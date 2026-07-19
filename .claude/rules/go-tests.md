---
paths:
  - "**/*_test.go"
---

# Règle — Conventions de test Go

Conventions toujours actives pour les tests Go de ce projet. La **démarche**
d'écriture relève de la skill `go-testing`.

## Exhaustivité & couverture — exigence forte

Objectif explicite de ce projet : **les tests couvrent à 100 % le code visé**,
instructions **et** branches. Ce n'est pas un chiffre décoratif, c'est
l'obligation d'exercer tout le comportement.

- **Toutes les branches** : chaque `if` / `else if` / `else`, chaque
  `switch` / `case` (`default` compris), chaque court-circuit `&&` / `||`, et
  chaque boucle (0 / 1 / n itérations) est exercé par au moins un cas.
- **Chemins non nominaux compris** : chaque retour d'erreur (`if err != nil`),
  chaque `panic` / `recover` et chaque cas dégénéré a son test dédié — pas
  seulement le chemin heureux.
- **Espace des entrées** (partitionnement d'équivalence + valeurs limites) :
  pour chaque paramètre, une valeur par classe **et** ses bornes — `0`, vide,
  `nil`, négatif, `math.MaxInt` / `MinInt` et dépassement, chaîne
  Unicode / multi-octets, `NaN` / `±Inf` pour les flottants, slice / map vide
  vs à un élément.
- **Vérification obligatoire** : `go test -cover` (au besoin `-coverprofile`
  puis `go tool cover -func`) confirme 100 % sur le code visé. Toute
  ligne / branche non couverte est soit testée, soit justifiée par un
  commentaire bref et vérifiable (ex. code réellement inatteignable).

## Organisation

- Un fichier `_test.go` colocalisé avec le code testé, jamais de dossier séparé.
- Boîte noire par défaut : `package xxx_test` pour valider l'API publique ;
  boîte blanche (`package xxx`) réservée aux invariants internes.

## Structure

- `TestXxx(t *testing.T)` ; sous-tests `t.Run(name, …)` aux noms lisibles.
- Table-driven dès que plusieurs cas partagent la même logique : slice (ou map
  pour l'indépendance d'ordre) de structs à champ `name`, variable `tt`.
- Corps de sous-test en **AAA** : *Arrange* (préparer entrées et fixtures),
  *Act* (**un seul** appel de l'unité testée), *Assert* (vérifier le résultat).
  Une seule action par cas ; pas de seconde assertion qui teste une autre unité.
- `t.Parallel()` sur tests et sous-tests indépendants (Go 1.22+).

## Assertions

- Pas de testify ni d'autre lib d'assertion : conditions Go explicites.
- `github.com/google/go-cmp/cmp` (`cmp.Diff` / `cmp.Equal`) pour les structures,
  pas `reflect.DeepEqual` ; comparaison directe pour les scalaires.
- Messages **got avant want**, fonction et entrée identifiées :
  `t.Errorf("Fn(%v) = %v, want %v", in, got, want)`.
- `t.Error*` par défaut ; `t.Fatal*` seulement dans un sous-test.

## Helpers & fixtures

- `t.Helper()` dans les helpers ; `t.TempDir()` et `t.Cleanup()` pour l'état
  jetable ; golden files (+ flag `-update`) pour les sorties volumineuses.

## Déterminisme (non négociable)

- Zéro dépendance au réseau, à l'horloge murale ou à l'ordre d'exécution.
- Horloge et dépendances injectées via interfaces ; fakes plutôt que mocks.
- `go test -race` doit passer.

## Erreurs

- Jamais de comparaison de message d'erreur par chaîne : `errors.Is` /
  `errors.As`, ou `err != nil` si le type importe peu.

## Spécifique à claude-benchy

- Les tests tournent dans le conteneur `go` du toolchain (`make test`, jamais un
  `go` hôte). `make check` (fmt → vet → lint → test) fait foi.
- **Ressources externes** de ce projet : le **daemon Docker**, le **réseau**, le
  **CLI `claude`** et l'**horloge**. Le code qui les pilote est isolé derrière
  des interfaces (`docker.Runner`) et des **fonctions pures** — tester
  celles-ci, jamais lancer un vrai conteneur ni appeler l'API dans un test.
  Exemples : la construction de la commande `docker run` (`internal/docker`), le
  parsing du flux `stream-json` et son rendu (`internal/claude`), la capture de
  diff et le chargement de spec sur fixtures.
- « Pas de Docker/réseau » vise le *pilotage* (lancer un conteneur, appeler
  l'API, écrire hors du `t.TempDir`), **pas le type entier**. Les **branches de
  garde et de cycle de vie** qui n'atteignent pas la ressource se testent et
  entrent dans la couverture : entrée invalide, chemin d'erreur, run **sans
  effet** (diff vide / aucun outil) et sa relance, retours anticipés. Injecter
  un fake `docker.Runner` plutôt que d'écarter la méthode.
- Sorties volumineuses (rapports `report.md`/`report.html`) : golden files sous
  `testdata/` avec flag `-update`, plutôt qu'assertions octet à octet dispersées.
- Build tags respectés : tests de code natif en `//go:build !js`.
