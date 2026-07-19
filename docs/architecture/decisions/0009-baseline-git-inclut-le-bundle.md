# ADR-0009 — La baseline git inclut le bundle

## Statut

Accepté

## Contexte

Après le passage de Claude, benchy capture un diff pour mesurer ce que la config a
produit. Si la baseline ne contenait que l'app nue, le diff surfacerait aussi les
fichiers du bundle superposé (le `CLAUDE.md`, les rules, les skills injectés) —
bruit qui n'est pas le travail de Claude.

## Décision

`workspace.Prepare` superpose le bundle **avant** de committer la baseline git.
La baseline contient donc app + bundle. Le diff ultérieur
(`diffcap.Capture`, `git diff --cached` vs baseline) ne montre alors que les
modifications faites par Claude.

## Conséquences

- Le diff isole proprement le travail de Claude, sans les fichiers de config
  injectés.
- La copie de l'app depuis un dépôt git n'embarque que l'arbre `HEAD` committé
  (ignorés/non suivis exclus), renforçant la reproductibilité.
- Toute modification du runner ou du workspace doit préserver cet ordre
  (superposition → commit → run → capture).
