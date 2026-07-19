# Glossaire

> Index canonique du vocabulaire. Un terme transverse est défini ici ; un terme
> propre à un contexte est défini dans sa page de contexte, référencée par lien.
> Chaque terme n'a **qu'une** source canonique.

## Termes transverses

- **benchy / claude-benchy** — le banc d'essai lui-même : l'outil qui compare des
  configurations de Claude Code sur un même prompt et une même app de test.
- **Claude Code** — l'assistant en ligne de commande dont benchy compare les
  configurations (via le CLI `claude`).
- **Configuration (config)** — une variante à comparer : un nom + un
  [bundle](banc/index.md) (+ éventuels *overrides* de modèle ou de prompt).
  Défini dans le contexte [banc](banc/index.md).
- **Artefacts** — les fichiers produits par un banc sous
  `results/<horodatage>/…` : `diff.patch`, `transcript.jsonl`, `result.json`,
  `checks.json`, `evaluation.json`, `report.md`, `report.html`, `bench.json`,
  `bench.yaml`.
- **Sandbox / bac à sable** — le conteneur Docker `--rm` non-root dans lequel
  Claude s'exécute pour un run. Défini dans le contexte [exécution](execution/index.md).

## Contextes métier

Le vocabulaire détaillé et les règles vivent dans les quatre contextes bornés :

- [**banc**](banc/index.md) — définir *quoi* comparer : fichier de banc, config,
  bundle, prompt, modèle, runs, `keepBaseConfig`.
- [**exécution**](execution/index.md) — *lancer* : workspace isolé, baseline,
  sandbox, run, run sans effet, relance, concurrence, isolation.
- [**capture**](capture/index.md) — *observer* : transcript stream-json,
  métriques, diff/patch, statistiques.
- [**évaluation**](evaluation/index.md) — *juger* : evaluate, rubric, check
  déterministe, juge LLM, critère, niveau, score, dispersion, classement,
  efficacité.
