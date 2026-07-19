---
titre: Définir un banc
public: tous
sources:
  - internal/spec/spec.go
  - internal/spec/paths.go
  - cmd/benchy/main.go
  - examples/bench.yaml
---

# Définir un banc

> Décrire, dans un fichier YAML, ce qu'on veut comparer : un prompt, une app de
> test, et plusieurs configurations concurrentes.

## User story

En tant qu'auteur de skills/rules, je veux décrire dans un fichier ce que je veux
comparer et comment, afin de lancer une comparaison reproductible d'une seule
commande.

## Le format du fichier de banc

```yaml
prompt: |                      # ou promptFile: ./prompts/x.md
  Ajoute un endpoint /health renvoyant 200.
app: ./app-under-test          # dossier de l'app de test (git non requis)
model: sonnet                  # défaut, surchargeable par config
runs: 1                        # répétitions par config (variance)
concurrency: 1                 # runs Docker en parallèle
retries: 2                     # relances d'un run « sans effet » ; 0 pour désactiver
auth:
  configDir: ~/.claude         # source des creds OAuth (défaut $CLAUDE_CONFIG_DIR|~/.claude)
sandbox:
  image: claude-benchy:latest  # image du bac à sable
output: ./results
keepBaseConfig: true           # garde le CLAUDE.md de l'app et y ajoute celui du bundle
evaluate:                      # optionnel (voir contexte évaluation)
  model: sonnet
  rubric: ["Expose GET /health renvoyant 200", "Modification minimale"]
  checks:
    - name: "endpoint déclaré"
      run: "grep -q '/health' server.js"
    - name: "fichier de test attendu"
      file: "server.test.js"
configs:
  - name: baseline
    bundle: ./configs/baseline
  - name: strict-rules
    bundle: ./configs/strict-rules
    model: opus                # override optionnel
    # prompt: ...              # override optionnel
```

Les chemins relatifs sont résolus par rapport au fichier de banc. Les défauts
appliqués : `model=sonnet`, `runs=1`, `concurrency=1`, `retries=2`,
`output=results`, `auth.configDir=$CLAUDE_CONFIG_DIR|~/.claude`,
`evaluate.model=model du banc`.

## Lancer le banc

```sh
./benchy run examples/bench.yaml
./benchy run --image claude-benchy-go:latest examples/bench.yaml
```

Un `--image` explicite l'emporte sur `sandbox.image` ; à défaut, `sandbox.image` ;
à défaut, l'image par défaut.

## Critères d'acceptation

- **Given** un banc valide, **When** on le charge, **Then** les défauts sont
  appliqués (sonnet/1/1/2/results) et les chemins relatifs absolutisés.
- **Given** un banc sans `app`, ou avec une config sans nom / sans bundle
  existant / sans prompt, **When** on le charge, **Then** le chargement échoue avec
  un message explicite.
- **Given** un banc avec un champ YAML inconnu, **When** on le charge, **Then** il
  est rejeté.
- **Given** une config sans prompt propre, **When** on la charge, **Then** elle
  hérite du prompt de tête ; une config avec `model` propre l'emporte.
- **Given** `prompt` et `promptFile` tous deux fournis, **When** on charge, **Then**
  l'erreur « mutuellement exclusifs » est levée.

Voir les règles [RG-banc-01 à RG-banc-04](../../domaine/banc/index.md#règles).

## Cas non triviaux & limites

- `retries` est tri-état : absent → 2, `0` → désactivé, négatif → invalide.
- Le fichier de banc soumis est **persisté** dans le dossier de résultats par
  `benchy run` (reproductibilité et reprise).
- L'app peut ne pas être un dépôt git ; si elle l'est, seul l'arbre `HEAD` est
  utilisé.
- Une image sandbox absente fait échouer le run avec `docker run: exit status 125`.

## Cas de test associés

`internal/spec/spec_test.go` : `TestLoadDefaultsAndOverrides`, `TestLoadRetries`,
`TestLoadPromptFile`, `TestLoadErrors`, `TestLoadEvaluate`,
`TestLoadEvaluateInvalidCheck`, `TestLoadMissingBundle`.
