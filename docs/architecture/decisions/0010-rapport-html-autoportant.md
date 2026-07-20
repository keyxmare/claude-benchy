# ADR-0010 — Rapport HTML autoportant (design Benchy inline)

## Statut

Accepté

## Contexte

Un rapport de banc doit pouvoir s'ouvrir et se partager comme un fichier unique,
y compris hors ligne, sans serveur ni dépendance. Il doit aussi porter une
identité visuelle (design system **Benchy** : sombre par défaut avec bascule
clair, accent jaune `#f5b301`, polices Space Grotesk / Inter / JetBrains Mono),
partagée avec le dashboard.

## Décision

`report.html` est **autoportant** : ses assets (CSS, JS de comparaison,
d'export et de thème) vivent dans des fichiers dédiés embarqués via `//go:embed`
puis **inlinés** dans le gabarit à la génération (`internal/report
/templates.go`), jamais liés par un `<link>`/`<script src>`. La comparaison côte
à côte fichier par fichier est calculée **dans le navigateur** (alignement LCS
en JavaScript embarqué) à partir des données injectées (`filesJSON` → variable
`BENCHY_FILES`). Les polices Space Grotesk / Inter / JetBrains Mono sont chargées
depuis Google Fonts avec repli sur les polices système hors-ligne. Le thème suit
`prefers-color-scheme` puis le choix persisté (`localStorage`), sans réseau.

## Conséquences

- Le rapport est un artefact unique, ouvrable directement, sans back-end.
- Les assets du rapport sont embarqués via `//go:embed` (`static/report.css`,
  `static/compare.js`, `static/export.js`, `static/theme.js`) — comme le
  dashboard (`internal/server`) — mais **inlinés** à la génération plutôt que
  liés, ce qui garde le rapport autoportant tout en respectant la règle « pas de
  CSS/JS en chaîne Go » (`.claude/rules/go-assets.md`).
- Le rendu de la comparaison est déporté côté client, ce qui garde le HTML
  interactif sans traitement serveur.
- Pour un usage strictement hors-ligne / RGPD, les polices peuvent être
  auto-hébergées / inlinées en `@font-face` (à demander).
