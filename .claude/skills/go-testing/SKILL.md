---
name: go-testing
description: Écrit ou complète des tests Go idiomatiques et déterministes. À invoquer dès qu'il faut ajouter, compléter ou corriger des tests dans un package Go — "écris les tests", "couvre ce package", "test de non-régression", "les tests manquent".
---

# Écrire des tests Go

Démarche pour produire des tests Go idiomatiques. Les conventions détaillées
(nommage, comparaisons, déterminisme) sont dans `.claude/rules/go-tests.md` et
s'appliquent tout du long.

## Quand l'invoquer

Toute tâche d'ajout, de complétion ou de correction de tests dans un package
Go. Pour un bug, cette skill impose le red-green.

## Démarche

1. **Cartographier** — lister les unités testables du périmètre, fonctions
   pures d'abord. Inclure aussi les **branches de garde** des méthodes dont le
   chemin nominal dépend d'une ressource externe (daemon Docker, réseau, CLI
   `claude`, horloge…) : entrée invalide, chemin d'erreur, teardown idempotent,
   validation de paramètres — testables sans la ressource (injecter un fake
   `docker.Runner`, opérer sur `t.TempDir`). Lire le code pour cerner le
   comportement réel, pas supposé ; ne pas écarter un type entier parce qu'une
   partie lance un conteneur ou appelle l'API.
2. **Choisir les cas — exhaustivement** — croiser trois angles :
   - *Boîte blanche (branches)* : lire le corps de l'unité et lister chaque
     branche — `if` / `else`, `switch` / `case` (+ `default`), `&&` / `||`,
     boucle (0 / 1 / n itérations), chaque `return`. Il faut un cas par branche.
   - *Boîte noire (partitionnement d'équivalence)* : une valeur représentative
     par classe d'entrée valide **et** invalide.
   - *Valeurs limites* : les bornes de chaque classe — `0`, vide, `nil`,
     négatif, `math.MaxInt` / `MinInt` et dépassement, chaîne
     Unicode / multi-octets, `NaN` / `±Inf`, off-by-one.
   Toujours inclure les **cas non nominaux** (erreurs, entrées invalides), pas
   seulement le chemin heureux. Un cas = une ligne de table. Cible : 100 % des
   branches (cf. règle `go-tests`).
3. **Poser la structure** — boîte noire (`package xxx_test`) par défaut ;
   table-driven + `t.Run(tt.name, …)` ; corps en **AAA** (Arrange / Act /
   Assert, avec un seul appel de l'unité en Act) ; `t.Parallel()` si le cas est
   indépendant.
4. **Écrire les assertions** — `cmp.Diff` pour les structures, comparaison
   directe pour les scalaires ; message `got`/`want` identifiant la fonction et
   l'entrée ; `t.Helper()` dans les helpers.
5. **Garantir le déterminisme** — aucune horloge murale, aucun réseau, aucun
   ordre implicite ; injecter les dépendances, utiliser des fakes.
6. **Exécuter et vérifier la couverture** — `go test -race -cover` jusqu'au
   vert, puis `go test -coverprofile=cover.out ./…` + `go tool cover -func` pour
   confirmer **100 % sur le code visé** (instructions et branches). Chaque ligne
   non couverte est soit couverte par un cas de plus, soit justifiée par un
   commentaire (code réellement inatteignable).
7. **Bug** — écrire d'abord le test qui échoue (rouge), puis le correctif
   minimal (vert).

## Squelette

```go
func TestScale(t *testing.T) {
    tests := []struct {
        name string
        in   Size
        want Size
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

## Checklist de complétude (avant de conclure)

- [ ] Chaque branche du code visé est exercée (`if`/`else`, `case`/`default`, `&&`/`||`, boucle 0/1/n).
- [ ] Chaque chemin d'erreur et cas non nominal a un cas dédié.
- [ ] Pour chaque entrée : classe valide, classe invalide, bornes (`0`, vide, `nil`, négatif, max/dépassement).
- [ ] `go tool cover -func` affiche 100 % sur le code visé (ou justification écrite pour l'inatteignable).
- [ ] `go test -race` passe ; tests déterministes (ni horloge, ni réseau, ni ordre).

## Pièges à éviter

- Tester la sortie formatée octet à octet plutôt que la sémantique.
- `reflect.DeepEqual` là où `cmp.Diff` donne un diff lisible.
- Comparer des messages d'erreur par chaîne.
- Des sous-tests parallèles qui partagent un état mutable.
