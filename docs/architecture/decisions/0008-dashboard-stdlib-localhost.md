# ADR-0008 — Dashboard bibliothèque standard, localhost par défaut

## Statut

Accepté

## Contexte

Le dashboard doit configurer, lancer et parcourir des bancs depuis le navigateur.
Or il lance des conteneurs Docker en montant des creds OAuth et, pour ses pickers,
il liste le système de fichiers de l'hôte. C'est puissant — et dangereux si exposé
au réseau.

## Décision

- Le serveur n'utilise **que** la bibliothèque standard (mux `net/http`
  méthode+motif de Go 1.22, `html/template`, `//go:embed` pour templates et
  assets) — aucune dépendance runtime hors `gopkg.in/yaml.v3`.
- La progression live passe par du **SSE** (canal fermé-remplacé pour réveiller
  les abonnés), pas de WebSocket ni de polling.
- **Localhost par défaut** (`--addr 127.0.0.1:80`) : la frontière de sécurité est
  l'adresse d'écoute. Les pickers (`/browse`, `/file`) et l'import
  (`/new?import=`) parcourent librement le FS de l'hôte, ce qui est **assumé** dans
  cette posture. Les routes portant sur des dossiers de résultats (`/report`,
  `/transcript`, `/apply`, `/export-config`, `/config-files`, `/new?from=`) sont en
  revanche confinées à la racine et rejettent le path traversal.
- Les actions `apply` et `export` écrivent **sans jamais committer**.

## Conséquences

- Déploiement léger, sans framework ni dépendance à surveiller.
- Ne **pas** exposer le dashboard au réseau : `--addr` ne s'ouvre à l'extérieur
  qu'en connaissance de cause.
- Docker-out-of-docker (dashboard conteneurisé) impose d'aligner les chemins hôte
  à l'identique (voir README, section Orbit).
- L'état des jobs en mémoire est perdu au redémarrage ; les artefacts restent
  listables via l'historique.
