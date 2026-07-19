# ADR-0005 — Export des creds depuis le Trousseau macOS

## Statut

Accepté

## Contexte

benchy monte les creds OAuth de Claude en lecture seule dans le sandbox. Or, sur
macOS, Claude Code range ses creds dans le **Trousseau**, pas dans
`~/.claude/.credentials.json` : monter le fichier directement échouerait.

## Décision

`spec.resolveCredsFile` calcule le chemin `configDir/.credentials.json`. Sur
`darwin`, il lit d'abord le Trousseau (`security find-generic-password -s "Claude
Code-credentials" -w`) et, en cas de succès, écrit le contenu dans ce fichier
(mode `0600`) avant de renvoyer le chemin. En cas d'échec de lecture, il retombe
sur le fichier existant sans le modifier. Hors darwin, il renvoie le chemin sans
jamais toucher au Trousseau. `runtime.GOOS` et le lecteur de Trousseau sont
indirectés par des variables pour rester testables hors macOS.

## Conséquences

- Le binaire natif (`benchy run`/`serve`) dispose toujours d'un fichier de creds à
  jour à chaque run sur macOS.
- Pour le dashboard conteneurisé, l'export doit être matérialisé au préalable
  (`make creds`, à relancer quand le token est rafraîchi), car son runner de
  tâches est un conteneur Linux sans accès au Trousseau.
- `resolveCredsFile` mêle volontairement commande (écrit le fichier) et requête
  (renvoie le chemin) : un compromis CQS justifié par le besoin d'export.
