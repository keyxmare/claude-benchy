# ADR-0001 — Runs sans effet : détection, relance, exclusion

## Statut

Accepté

## Contexte

Un run Claude peut « réussir » côté CLI (aucune erreur) tout en ne produisant
rien : aucun appel d'outil, ou un diff vide — par exemple quand le modèle écrit un
appel de sous-agent en texte au lieu de l'exécuter et termine la session. Ce
**faux succès** fausserait les moyennes (un 0/100 parasite) et les classements
d'efficacité (un run à ~0 s / ~0 $).

## Décision

Un run est **dégénéré** (« sans effet ») quand il n'a pas d'erreur mais que
`Metrics.ToolUses == 0` **ou** `Diff.FilesChanged == 0`
(`report.RunReport.Degenerate()`). `OK()` exige l'absence d'erreur, l'absence
d'erreur Claude et la non-dégénérescence — dégénéré et OK sont mutuellement
exclusifs.

Le runner tente `1 + RetryCount()` fois (défaut retries = 2, soit 3 tentatives) ;
il s'arrête sur une erreur dure (non relancée) ou au premier résultat non
dégénéré ; chaque relance repart d'un workspace neuf. Un run resté dégénéré est
marqué « sans effet » et **écarté** de toutes les agrégations (score, efficacité,
juge) tout en restant visible dans la comparaison et le détail. `retries: 0`
désactive la relance.

## Conséquences

- Les moyennes et classements ne sont plus pollués par des faux succès.
- Le juge ne voit que les runs `OK()` (sinon il injecterait un 0/100 trompeur).
- Coût supplémentaire : jusqu'à `retries` runs Claude en plus par config atteinte.
- Une erreur dure n'est jamais relancée — seule la dégénérescence l'est.
- Invariante centrale à préserver lors de toute évolution du runner et du report.
