---
paths:
  - "**/*.go"
---

# Règle — Assets front hors des sources Go

Conventions toujours actives pour tout code Go de ce projet qui produit du HTML
(rapports `internal/report`, dashboard `internal/server`).

## Pas de CSS dans une string Go

Aucune feuille de style — ni bloc `<style>`, ni règles CSS — n'est écrite dans
une string Go (constante, littéral, `text/template`/`html/template` inline). Le
CSS vit dans un fichier `.css` dédié, embarqué à la compilation via `//go:embed`.

Raisons : l'éditeur retrouve coloration, autocomplétion et lint CSS ; les gates
CSS peuvent s'appliquer au fichier ; le diff d'un changement de style ne pollue
pas le code Go ; la string géante (guillemets à échapper, pas de repli) disparaît.

## L'alternative : `//go:embed`

C'est le mécanisme standard, **déjà utilisé** ici par `internal/server` :

```go
import _ "embed"

//go:embed static/style.css
var styleCSS string
```

Puis on injecte la variable dans le template (via un champ de données ou une
fonction de template), au lieu d'écrire le CSS en dur dans `htmlSource`.

- Un `//go:embed` par asset (ou `embed.FS` pour un dossier `static/`), à côté du
  code qui le sert — jamais un chemin absolu ni une lecture disque à l'exécution.
- Le rapport HTML doit rester **autonome** (fichier unique ouvrable hors serveur) :
  le CSS embarqué est **inliné** dans un `<style>` à la génération, pas référencé
  par un `<link>` vers un fichier voisin.
- Même principe pour le JavaScript et les gros fragments HTML statiques : fichier
  dédié + `//go:embed`, pas de littéral Go à rallonge.

## Portée

- Vise le CSS/JS/HTML **statique** (mise en forme, structure fixe). Le HTML
  **dynamique** piloté par les données reste un `text/template`/`html/template`
  idiomatique — ce n'est pas visé.
- Les fichiers `.css` embarqués suivent le design system Gazoline déjà en place
  (cf. l'en-tête de `internal/report/static/report.css`).
