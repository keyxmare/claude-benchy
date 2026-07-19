---
titre: internal/claude — parsing et rendu du transcript
public: dev
sources:
  - internal/claude/stream.go
  - internal/claude/render.go
---

# internal/claude — parsing et rendu du transcript

> Lit le flux `stream-json` émis par `claude -p` : d'un côté il en extrait les
> métriques finales du run, de l'autre il rend chaque ligne en texte lisible pour
> la sortie live et la page transcript.

## Rôle & responsabilités

- `stream.go` : `Parse(r)` produit les `Metrics` du run (coût, tokens, tours,
  erreur…) et **dérive** le nombre d'appels d'outils depuis le flux.
- `render.go` : `Render(line)` transforme **une** ligne de transcript en zéro ou
  plusieurs lignes lisibles ; `LiveWriter` bufferise des écritures partielles,
  découpe sur les sauts de ligne et émet les lignes rendues à la volée.

## Flux principaux

**`Parse`** parcourt le flux ligne par ligne (buffer jusqu'à `maxLineBytes` =
16 Mio pour tolérer une grosse ligne JSON). Les événements `assistant` sont
inspectés : chaque bloc `tool_use` incrémente `ToolUses` et le compteur par outil
`ToolBreakdown`. L'événement `result` final est démarshalé en `Metrics`. À la fin,
`ToolUses`/`ToolBreakdown` (non trusté du result event) sont attachés.

- Une ligne non-JSON (hors result) est **tolérée** (ignorée) ;
- une ligne `result` corrompue est **fatale** ;
- **aucun événement `result`** → erreur `"no result event found in transcript"` :
  c'est ainsi qu'un run vide est signalé en amont.

**`Render`** commute sur le `type` : `system` → rien (bannière d'init écartée),
`assistant` → texte + appels d'outils préfixés `⏺ ` (`Bash`, `Edit`, `Read`,
`Write`, `Grep`, `Glob`, générique), `user` → résultats d'outils préfixés `  ⎿ `
(`⚠` sur erreur), `result` → ligne finale `✓ terminé · N tours · $coût` ou
`✗ échec`. Toute ligne inconnue/vide/parasite → `nil`. Les champs longs sont
tronqués de façon **rune-safe** (`truncate`).

**`LiveWriter`** : `Write` accumule et émet chaque ligne complète ; `Flush` émet
un reliquat non terminé par un saut de ligne. Gère un événement scindé sur
plusieurs `Write`.

## Dépendances & cibles

Bibliothèque standard (`encoding/json`, `bufio`). Consommé par `internal/runner`
(métriques + tee live vers `Options.Agent`) et par `internal/server` (rejeu de la
page `/transcript`).

## Invariants & pièges

- Un transcript **doit** contenir un événement `result` terminal.
- `Render` est total et ne panique jamais : une ligne non reconnue donne `nil`.
- Tolérance ligne parasite (skip) mais un `result` corrompu est fatal
  (asymétrie volontaire).
- Troncature rune-safe (jamais au milieu d'un caractère UTF-8).

## Cas de test associés

`stream_test.go` : `TestParse`, `TestParseNoResult`,
`TestParseIgnoresGarbageLines`.
`render_test.go` : `TestRenderAssistantTextAndTools`,
`TestLiveWriterSplitsAndFlushes`, `TestTruncateRuneSafe`.
