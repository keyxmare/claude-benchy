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

1. **Cartographier** — lister les unités testables du périmètre, en
   commençant par les fonctions pures. Lire le code pour cerner le comportement
   réel, pas supposé.
2. **Choisir les cas** — pour chaque unité : cas nominal, limites (zéro, vide,
   négatif, max), cas d'erreur, cas dégénérés. Un cas = une ligne de table.
3. **Poser la structure** — boîte noire (`package xxx_test`) par défaut ;
   table-driven + `t.Run` ; `t.Parallel()` si le cas est indépendant.
4. **Écrire les assertions** — `cmp.Diff` pour les structures, comparaison
   directe pour les scalaires ; message `got`/`want` identifiant la fonction et
   l'entrée ; `t.Helper()` dans les helpers.
5. **Garantir le déterminisme** — aucune horloge murale, aucun réseau, aucun
   ordre implicite ; injecter les dépendances, utiliser des fakes.
6. **Exécuter et itérer** — `go test ./... -race -cover` jusqu'au vert ;
   viser une couverture utile des branches, pas un chiffre.
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

## Pièges à éviter

- Tester la sortie formatée octet à octet plutôt que la sémantique.
- `reflect.DeepEqual` là où `cmp.Diff` donne un diff lisible.
- Comparer des messages d'erreur par chaîne.
- Des sous-tests parallèles qui partagent un état mutable.
