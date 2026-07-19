---
titre: internal/diffcap — capture du diff
public: dev
sources:
  - internal/diffcap/diffcap.go
---

# internal/diffcap — capture du diff

> Capture les changements du workspace (après passage de Claude) sous forme de
> diff unifié contre la baseline git, avec ses statistiques.

## Rôle & responsabilités

`Capture(dir string) (Result, error)` stage tous les changements (`git add -A`,
nouveaux fichiers compris) puis renvoie :

- `Patch` : la sortie de `git diff --cached` ;
- `Stats` : `FilesChanged`, `Insertions`, `Deletions`, dérivés de
  `git diff --cached --numstat`.

## Flux principaux

`Capture` suppose un dépôt git avec une baseline (fournie par
`internal/workspace`). `parseNumstat` découpe la sortie numstat par ligne, en
tabulations ; chaque ligne valide incrémente `FilesChanged` ; les colonnes
insertions/suppressions sont converties en entiers (un `-` de fichier binaire est
ignoré silencieusement, mais le fichier reste compté). Le helper `git` préfixe
`-c safe.directory=* -C <dir>` (git en root sur montage bind).

## Dépendances & cibles

Bibliothèque standard + binaire `git`. Consommé par `internal/runner` après
chaque run réussi.

## Invariants & pièges

- `git add -A` capture ajouts, modifications et suppressions uniformément : le
  diff reflète le delta complet vs baseline.
- Une ligne numstat non numérique (binaire `-`) compte tout de même comme fichier
  changé mais contribue 0 insertion/suppression.
- `safe.directory=*` requis pour un workspace bind-monté appartenant à un autre
  utilisateur.

## Cas de test associés

`diffcap_test.go` : `TestCaptureChanges`, `TestCaptureNoChanges`.
