# ADR-0010 — Rapport HTML autoportant (design Gazoline inline)

## Statut

Accepté

## Contexte

Un rapport de banc doit pouvoir s'ouvrir et se partager comme un fichier unique,
y compris hors ligne, sans serveur ni dépendance. Il doit aussi porter une
identité visuelle (design system **Gazoline** de Motoblouz : noir/blanc, accent
jaune `#f1ab00`, Montserrat).

## Décision

`report.html` est **autoportant** : son CSS est inline dans le gabarit
(`internal/report/templates.go`), et la comparaison côte à côte fichier par
fichier est calculée **dans le navigateur** (alignement LCS en JavaScript embarqué)
à partir des données injectées (`filesJSON` → variable `BENCHY_FILES`). Les polices
Montserrat/Inter sont chargées depuis Google Fonts avec repli sur les polices
système hors-ligne.

## Conséquences

- Le rapport est un artefact unique, ouvrable directement, sans back-end.
- Le CSS du rapport vit dans une chaîne Go plutôt que dans un `.css` embarqué —
  contrairement au dashboard (`internal/server`) qui embarque `static/style.css`
  via `//go:embed`. Cette différence est assumée pour garder le rapport
  autoportant.
- Le rendu de la comparaison est déporté côté client, ce qui garde le HTML
  interactif sans traitement serveur.
- Pour un usage strictement hors-ligne / RGPD, les polices peuvent être
  auto-hébergées / inlinées en `@font-face` (à demander).
