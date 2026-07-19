---
titre: internal/spec — chargement et validation du banc
public: dev
sources:
  - internal/spec/spec.go
  - internal/spec/paths.go
  - internal/spec/creds.go
  - internal/spec/import.go
---

# internal/spec — chargement et validation du banc

> Charge un fichier de banc YAML en une `Spec` validée, résout ses chemins
> relatifs, localise les creds OAuth, et sait ré-émettre un banc en chemins
> absolus sans le dénaturer.

## Rôle & responsabilités

Ce package transforme un fichier de banc en une structure `Spec` **complète,
défautée et validée**, prête pour le runner. Il :

- lit et décode le YAML avec `KnownFields(true)` (un champ inconnu est une
  erreur, `spec.go`) ;
- applique les valeurs par défaut et résout les chemins relatifs au fichier de
  banc (`resolve`) ;
- valide la cohérence (`validate`) ;
- localise le fichier de creds OAuth à monter dans le sandbox (`creds.go`),
  avec un export depuis le Trousseau sur macOS ;
- fournit `Import`, qui réécrit les chemins d'un banc en absolu **sans** appliquer
  de défaut ni de validation (le résultat reste un point de départ éditable).

Il **ne** lance rien (pas de conteneur, pas de juge, pas de checks) : `Check` et
`Evaluate` sont ici seulement parsés et validés, exécutés ailleurs.

## Flux principaux

`Load(path)` lit le fichier, décode le YAML, calcule `baseDir` (répertoire absolu
du fichier de banc) puis appelle `Build(baseDir)` = `resolve` + `validate`.
`Build` est séparé de `Load` pour qu'une `Spec` construite en mémoire (formulaire
web) passe par le **même** pipeline de résolution/validation.

Valeurs par défaut appliquées par `resolve` :

| Champ | Défaut |
|---|---|
| `model` | `sonnet` |
| `runs` | `1` |
| `concurrency` | `1` |
| `retries` | `2` (via `RetryCount()` quand `Retries` est nil) |
| `output` | `results` |
| `auth.configDir` | `$CLAUDE_CONFIG_DIR`, sinon `~/.claude` |
| `evaluate.model` | le `model` du banc |

Résolution des chemins : `app`, `output`, `promptFile` sont absolutisés contre
`baseDir` ; pour chaque config, `bundle` et `promptFile` aussi. Un chemin déjà
absolu ou vide est laissé tel quel ; `~` est étendu via le home de l'utilisateur.

Le prompt d'une config vide **hérite** du prompt de tête ; le `model` d'une config
vide hérite du `model` du banc.

## Dépendances & cibles

Dépend de `gopkg.in/yaml.v3` (décodage strict pour `Load`, arbre de nœuds
lossless pour `Import`) et de la bibliothèque standard. `runtime.GOOS` et le
lecteur de Trousseau sont indirectés par des variables pour rester testables hors
macOS.

## Invariants & pièges

- **Un banc est invalide** sauf s'il a un `app` non vide pointant un dossier
  existant, au moins une config, chaque config avec un nom unique non vide dont le
  `bundle` existe et avec un prompt résoluble. Voir
  [RG-banc-01…](../../domaine/banc/index.md).
- **`prompt` et `promptFile` sont mutuellement exclusifs** à tout niveau.
- **`retries` en `*int`** : nil → défaut 2 ; `0` explicite → désactivé ; négatif →
  invalide. Le pointeur distingue « non renseigné » de « 0 explicite ».
- **Champs YAML inconnus rejetés** (`KnownFields(true)`) : échec immédiat sur une
  faute de frappe.
- `Import` préserve commentaires, ordre et valeurs ; il ne réécrit que les
  scalaires de chemin relatifs — voir [ADR-0005](../decisions/0005-export-creds-trousseau-macos.md)
  pour la localisation des creds et le fichier `import.go`.
- La localisation des creds mélange volontairement commande (écrit le fichier
  exporté du Trousseau) et requête (renvoie le chemin) — voir
  [ADR-0005](../decisions/0005-export-creds-trousseau-macos.md).

## Cas de test associés

`spec_test.go` : `TestLoadDefaultsAndOverrides`, `TestLoadRetries`,
`TestLoadPromptFile`, `TestLoadErrors`, `TestLoadEvaluate`,
`TestLoadEvaluateInvalidCheck`, `TestLoadMissingBundle`.
`import_test.go` : `TestImportAbsolutizesRelativePaths`,
`TestImportPreservesComments`,
`TestImportLeavesEmptyAndConfigsNonSequenceUntouched`, `TestImportErrors`.
`import_internal_test.go` : `TestDocumentMapping`, `TestMapValue`,
`TestAbsolutizeScalar`.
`creds_test.go` : `TestResolveCredsFileDarwinExportsKeychain`,
`TestResolveCredsFileDarwinKeychainMissKeepsExistingFile`,
`TestResolveCredsFileNonDarwinNeverReadsKeychain`.
