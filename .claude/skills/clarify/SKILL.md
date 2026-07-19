---
name: clarify
description: Lève l'ambiguïté de toute demande avant d'agir — interroge sans relâche, une question à la fois, pour fixer le QUOI et le POURQUOI (objectif, périmètre, critères) avant tout COMMENT. Pour un besoin métier, confronte vocabulaire et règles à `docs/domaine/` et y capture glossaire, règles et US au fil de l'eau. À invoquer dès qu'une demande est floue — "clarifie", "cadre le besoin", "qu'est-ce qu'on fait exactement", "/clarify <sujet>" — y compris une tâche outillage ou méta. Implémenter relève de `feature`, corriger un bug de `fix` — qui l'invoquent en amont.
argument-hint: <demande ou sujet à cadrer>
allowed-tools: Task, AskUserQuestion, Read, Grep, Glob, Edit, Write, Bash(git:*)
---

# clarify — lever l'ambiguïté avant d'agir

Cœur, valable pour TOUTE demande : interroger jusqu'à fixer le QUOI et le
POURQUOI — objectif, périmètre, critères d'acceptation — avant tout COMMENT
technique (qui relève de `feature` §2, plan mode). Un besoin métier s'ancre en
plus dans `docs/domaine/` et y est capturé ; une tâche outillage ou méta suit
le même questionnement mais se restitue dans la réponse, sans `docs/domaine/`.
Read partout, écriture dans `docs/`.

## 1. Indexer la référence

Ancrer les questions dans le réel pour y détecter les contradictions. Besoin
métier → lire `docs/domaine/` (`glossaire.md`, `regles-metier.md`,
`user-stories.md`), recenser termes, règles et US fixés ; doc
absente → la créer fait partie du cadrage ; domaine large → déléguer le
repérage dans le code à un agent Explore. Tâche outillage/méta → la référence
est le code, la spec ou les conventions concernés.

## 2. Interroger jusqu'à lever toute ambiguïté

Cœur du skill : **une seule question à la fois**, ancrée dans la référence
réelle, tant qu'un flou subsiste — jamais grouper, supposer une réponse, ni se
contenter d'un vague. Choix discrets et tranchés → `AskUserQuestion` (une
question par appel).

- Expliciter objectif, valeur, acteurs, périmètre (dans / hors) et critères
  d'acceptation observables.
- Inventer cas limites et états interdits pour tester les frontières.
- Besoin métier : confronter chaque terme au glossaire (conflit ou synonyme →
  trancher le canonique) ; creuser règles de gestion et invariants (montants,
  états et transitions, unicité, RGPD), bannir toute formulation floue.

S'arrêter au QUOI/POURQUOI. Une question de COMMENT → la noter pour `feature`
§2, pas ici. Un point tranché ne se rouvre pas sans élément nouveau.

## 3. Capturer au fil de l'eau (besoin métier)

Dès qu'un point métier est tranché, l'écrire — jamais en fin de session,
cohérence locale avec le style des docs :

- terme → `glossaire.md` (définition canonique, synonymes bannis) ;
- règle ou invariant → `regles-metier.md` ;
- besoin → `user-stories.md` : « en tant que… je veux… afin de… », avec
  critères d'acceptation et cas limites du §2.

Contradiction avec une règle existante → remonter, trancher avec
l'utilisateur, mettre à jour `regles-metier.md`.

## Sortie

Demande sans ambiguïté résiduelle, résiduel assumé. Besoin métier →
`docs/domaine/` à jour (glossaire, règles, US avec critères), restitué en
diff. Tâche outillage/méta → cadrage restitué dans la réponse. Invoqué depuis
`feature` ou `fix` → rendre la main avec ce cadrage.
