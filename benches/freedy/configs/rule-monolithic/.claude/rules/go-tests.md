# Règle — Tests Go

Règle unique et complète pour l'écriture des tests Go de ce projet. Elle couvre
à la fois la démarche et les conventions.

## Démarche

1. Repérer les unités à tester, en commençant par les fonctions pures ; cerner
   le comportement attendu (nominal, limites, erreurs, cas dégénérés).
2. Écrire les tests, les lancer, itérer jusqu'au vert.
3. Pour un bug : test de non-régression **rouge d'abord**, puis correctif
   (red-green).

## Organisation

- Un fichier `_test.go` colocalisé avec le code testé, jamais de dossier de
  tests séparé.
- Boîte noire par défaut pour valider l'API publique : `package xxx_test`.
  Boîte blanche (`package xxx`) réservée aux invariants internes.

## Structure

- Fonctions `TestXxx(t *testing.T)` ; sous-tests via `t.Run(name, …)` avec des
  noms lisibles décrivant le cas.
- Tests **table-driven** dès qu'une même logique couvre plusieurs cas : une
  slice (ou une map pour forcer l'indépendance d'ordre) de structs avec un
  champ `name` ; variable de boucle `tt`.
- `t.Parallel()` sur les tests et sous-tests indépendants (Go 1.22+ : la
  variable de boucle n'a plus à être recapturée).

```go
func TestScale(t *testing.T) {
    tests := []struct {
        name     string
        in       Size
        want     Size
    }{
        {"square", Size{10, 10}, Size{1, 1}},
        {"wide", Size{20, 10}, Size{2, 1}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            if got := Scale(tt.in); got != tt.want {
                t.Errorf("Scale(%v) = %v, want %v", tt.in, got, tt.want)
            }
        })
    }
}
```

## Assertions & comparaisons

- **Pas de bibliothèque d'assertion** (pas de testify) : des conditions Go
  explicites.
- Comparer des structures entières avec `github.com/google/go-cmp/cmp`
  (`cmp.Diff` / `cmp.Equal`), pas `reflect.DeepEqual`. Pour des scalaires, la
  comparaison directe suffit.
- Message d'échec : identifier la fonction et l'entrée, **got avant want** :
  `t.Errorf("Fn(%v) = %v, want %v", in, got, want)`.
  Avec cmp : `t.Errorf("Fn(%v) mismatch (-want +got):\n%s", in, cmp.Diff(want, got))`.
- Préférer `t.Error*` (poursuit le test) à `t.Fatal*` ; `t.Fatal*` reste
  acceptable dans un sous-test.

## Helpers, fixtures, nettoyage

- `t.Helper()` dans toute fonction d'assistance.
- `t.TempDir()` pour les fichiers temporaires, `t.Cleanup()` pour le nettoyage.
- Golden files pour les sorties volumineuses ; comparer sémantiquement (parser)
  plutôt que des octets bruts ; prévoir un flag `-update`.

## Déterminisme (non négociable)

- Aucun test ne dépend du réseau, de l'horloge murale ni de l'ordre
  d'exécution.
- Injecter horloge et dépendances via des interfaces ; préférer des **fakes**
  aux mocks.
- `go test -race` doit passer.

## Erreurs

- Ne pas comparer les messages d'erreur par chaîne (fragile). Tester la
  sémantique : `errors.Is` / `errors.As`, ou simplement `err != nil` quand le
  type importe peu.

## Outils

- `go test ./...` fait foi ; `go test -cover` pour la couverture, `go test
  -race` pour la concurrence.
- Fuzzing (`FuzzXxx` + `testing.F`) pour le parsing et les fonctions pures à
  large domaine d'entrée.

## Spécifique à freedy

- `go test` tourne **sur l'hôte** (dérogation Docker du projet).
- `internal/renderer` est agnostique de plateforme : tester les fonctions pures
  (géométrie, mise à l'échelle, formats), jamais GPU / GLFW / `syscall/js`.
- Respecter les build tags : les tests de code natif portent `//go:build !js`.
