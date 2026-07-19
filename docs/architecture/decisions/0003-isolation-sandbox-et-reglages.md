# ADR-0003 — Isolation du bac à sable et des réglages

## Statut

Accepté

## Contexte

benchy lance Claude en autonomie totale (`--dangerously-skip-permissions`) pour
mesurer une config sans intervention. Il faut à la fois contenir les effets de
bord et garantir que le résultat mesure **la config du banc**, pas les réglages
personnels de l'opérateur sur l'hôte.

## Décision

- Un conteneur `--rm` **jetable** par run, exécuté en utilisateur **non-root**
  (imposé par le `USER` des Dockerfiles, pas par un flag `--user`).
- Claude tourne avec `--dangerously-skip-permissions` : le conteneur **est** le
  bac à sable ; le seul accès disque est la copie montée de l'app. Le réseau reste
  ouvert (l'API Claude en a besoin).
- Isolation des réglages via `--setting-sources project,local` (ajouté par le
  runner) : les settings/skills utilisateur de l'hôte ne contaminent pas le test.
- `internal/docker` ne monte que le workspace et, en lecture seule, le fichier de
  creds OAuth — rien d'autre de l'hôte.

## Conséquences

- Les runs sont reproductibles et sans effet de bord persistant.
- Le résultat reflète le bundle testé, pas la config hôte de l'opérateur.
- La responsabilité est répartie : `internal/docker` isole par omission (montages
  minimaux), le runner ajoute les flags Claude — `internal/docker` reste agnostique
  de Claude et testable comme fonction pure.
- Le réseau ouvert est un compromis assumé (dépendance à l'API).
