# ADR-0002 — `keepBaseConfig` : remplacer vs fusionner le `CLAUDE.md`

## Statut

Accepté

## Contexte

Un bundle de config est superposé tel quel sur la copie de l'app. Quand le bundle
et l'app portent tous deux un `CLAUDE.md`, il faut choisir : la surcouche
remplace-t-elle la base du projet, ou s'y ajoute-t-elle ? Les deux comparaisons
sont légitimes (« surcouche seule » vs « base + surcouche »).

## Décision

Par défaut (`keepBaseConfig: false`), un fichier du bundle **remplace** celui de
l'app au même chemin — le `CLAUDE.md` du bundle écrase donc celui de l'app. Avec
`keepBaseConfig: true`, le `CLAUDE.md` de l'app est **conservé comme base** et
celui du bundle lui est **ajouté à la suite** (séparés par une ligne vide).

La fusion ne concerne **que** la collision d'un `CLAUDE.md` : tous les autres
fichiers du bundle (`.claude/rules/*`, `.claude/skills/*`…) écrasent toujours. Si
l'app n'a pas de `CLAUDE.md`, celui du bundle est utilisé tel quel même avec la
fusion active. La décision est portée par le booléen `mergeClaudeMd` de
`workspace.Prepare`.

## Conséquences

- On peut comparer soit la surcouche seule, soit son effet cumulé avec la base du
  projet, sans dupliquer les bundles.
- Le comportement est asymétrique (spécial `CLAUDE.md`) : un lecteur doit le
  savoir pour interpréter un diff.
- Exposé dans l'interface web par la case « Conserver la config de base de l'app ».
