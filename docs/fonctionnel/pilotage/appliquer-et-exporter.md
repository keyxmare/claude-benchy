---
titre: Appliquer et exporter
public: tous
sources:
  - internal/server/apply.go
  - internal/server/exportconfig.go
---

# Appliquer et exporter

> Adopter en un clic la sortie d'une config (son diff) ou sa configuration (les
> fichiers de son bundle), le tout laissé non committé pour relecture.

## User story

En tant qu'utilisateur, je veux adopter la production ou la config d'une variante
gagnante sur mon projet, afin de matérialiser le résultat d'un banc sans copier à
la main.

## Les deux actions

- **Appliquer au projet** : depuis le rapport, « Appliquer au projet » applique le
  diff d'une config (`git apply`) sur l'app testée, laissé **non committé** pour
  relecture.
- **Exporter la config** : copier dans l'app les fichiers sélectionnés du bundle
  d'une config (également non committé). Le message final s'accorde en nombre
  (« 1 fichier de conf exporté » / « N fichiers de conf exportés »).

Ces actions n'apparaissent que sur un rapport servi par le dashboard (pas sur un
rapport autonome).

## Critères d'acceptation

- **Given** un run avec un `diff.patch`, **When** on clique « Appliquer au
  projet », **Then** le diff est appliqué à l'app (non committé) et un message de
  succès s'affiche.
- **Given** un patch qui ne s'applique pas, **When** on applique, **Then** l'échec
  est signalé (422) avec la sortie de `git apply`.
- **Given** des fichiers de bundle sélectionnés, **When** on exporte, **Then** ils
  sont copiés dans l'app avec un message accordé au nombre.
- **Given** un chemin `..`, absolu, ou une cible hors du dossier de résultats,
  **When** on applique/exporte, **Then** la requête est rejetée.

## Cas non triviaux & limites

- Rien n'est jamais committé : les changements restent en attente de relecture.
- L'export refuse les chemins absolus ou contenant `..`.
- Un run sans bundle connu dans ses artefacts ne peut pas exporter (« bundle
  inconnu pour ce run »).

## Cas de test associés

`internal/server/apply_test.go` : `TestApplyAppliesPatchToProject`,
`TestApplyErrors`.
`internal/server/exportconfig_test.go` : `TestConfigFilesLists`,
`TestConfigFilesErrors`, `TestConfigFilesRejectsBadArtifact`,
`TestExportConfigCopiesSelectedFiles`, `TestExportConfigSingleFileMessage`,
`TestExportConfigErrors`.
`internal/report/report_test.go` : `TestApplyButtonOnlyWhenServed`.
