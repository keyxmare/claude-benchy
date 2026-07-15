# Règle — Conventions de test Go

Conventions toujours actives pour les tests Go de ce projet. La **démarche**
d'écriture relève de la skill `go-testing`.

## Organisation

- Un fichier `_test.go` colocalisé avec le code testé, jamais de dossier séparé.
- Boîte noire par défaut : `package xxx_test` pour valider l'API publique ;
  boîte blanche (`package xxx`) réservée aux invariants internes.

## Structure

- `TestXxx(t *testing.T)` ; sous-tests `t.Run(name, …)` aux noms lisibles.
- Table-driven dès que plusieurs cas partagent la même logique : slice (ou map
  pour l'indépendance d'ordre) de structs à champ `name`, variable `tt`.
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

## Spécifique à freedy

- `go test` tourne sur l'hôte (dérogation Docker du projet).
- `internal/renderer` est agnostique de plateforme : tester les fonctions pures,
  jamais GPU / GLFW / `syscall/js`.
- Build tags respectés : tests de code natif en `//go:build !js`.
