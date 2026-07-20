# Règles métier

> Index des règles à identifiant stable. Chaque règle est définie dans sa page de
> contexte (source canonique) et référencée ici. Les pages d'architecture et de
> fonctionnel renvoient à ces identifiants sans les dupliquer.

## Contexte banc — voir [banc](banc/index.md)

- **RG-banc-01** — Un banc est invalide sans `app` (dossier existant), sans au
  moins une config, ou avec une config sans nom unique / sans bundle existant /
  sans prompt résoluble.
- **RG-banc-02** — Le prompt est donné en clair (`prompt`) ; une config sans
  prompt hérite du prompt de tête.
- **RG-banc-03** — `retries` : non renseigné → défaut 2 ; `0` explicite →
  désactivé ; négatif → invalide.
- **RG-banc-04** — Un champ YAML inconnu rend le banc invalide.
- **RG-banc-05** — `keepBaseConfig` ne fusionne que le `CLAUDE.md` en collision ;
  les autres fichiers du bundle écrasent toujours.

## Contexte exécution — voir [exécution](execution/index.md)

- **RG-exec-01** — Un run est *sans effet* (dégénéré) s'il n'a pas d'erreur mais
  n'a fait aucun appel d'outil **ou** n'a produit aucun changement de fichier.
- **RG-exec-02** — Un run dégénéré est relancé jusqu'à `retries` fois ; une erreur
  dure n'est jamais relancée.
- **RG-exec-03** — Un run resté dégénéré est écarté de toutes les agrégations, mais
  reste visible dans la comparaison et le détail.
- **RG-exec-04** — Chaque run tourne dans un conteneur jetable non-root ne montant
  que la copie de l'app et les creds OAuth ; les réglages/skills de l'hôte sont
  isolés.
- **RG-exec-05** — La baseline git inclut le bundle : le diff ne surface que les
  changements de Claude.

## Contexte capture — voir [capture](capture/index.md)

- **RG-capture-01** — Un transcript doit contenir un événement `result` terminal ;
  son absence signale un run vide.
- **RG-capture-02** — Le nombre d'appels d'outils est dérivé du flux (compté sur
  les blocs `tool_use`), pas lu du result event.

## Contexte évaluation — voir [évaluation](evaluation/index.md)

- **RG-eval-01** — L'évaluation ne tourne que si un rubric **ou** des checks sont
  déclarés ; un check doit fixer `run` **ou** `file`.
- **RG-eval-02** — Le juge n'examine que les runs exploitables (`OK()`) ; il est
  sauté si aucun run n'est OK.
- **RG-eval-03** — Le score est calculé en Go depuis les niveaux par critère
  (respecté = 1, partiel = 0,5, non = 0), jamais fourni par le modèle.
- **RG-eval-04** — L'évaluation est agrégée par config : score moyen des runs +
  dispersion (min–max) ; le tri se fait par score moyen décroissant.
- **RG-eval-05** — Le modèle juge vaut, par défaut, le modèle du banc.
