# AGENTS.md — internal/rules/

> Instructions spécifiques à `internal/rules/` — à lire **en plus** de la racine.

## Responsabilité

**Moteur de règles YAML** : détecter les anti-patterns de performance et
produire des **findings** (avec suggestions de fix).

- Parser les fichiers YAML de règles (`proto/rules.example.yaml` comme modèle).
- **Matcher** : comparer chaque règle contre le `CallGraph` de `analyzer/`.
- Générer des `Finding` (ID de la règle, localisation, valeur mesurée,
  recommendation textuelle).

## Format de règle YAML (référence → `proto/rules.example.yaml`)

```yaml
rules:
  - id: <string, unique ^[a-z0-9-]+$>
    name: <string>
    description: <string>
    severity: critical|high|medium|low
    effort: low|medium|high
    controllability: controllable|partial|none
    match:
      function_pattern: <regex>            # regexp sur le callee
      exclude_pattern: <regex>             # callee ignoré (anti faux positifs)
      call_count_threshold: ">=N"          # par site d'appel (heuristique in-loop)
      caller_count_threshold: ">=N"        # sites d'appel distincts du callee
      memory_per_call_threshold_mb: ">=X"  # ΣMU/ΣCT tous sites confondus
      peak_memory_per_call_threshold_mb: ">=X" # ΣPMU/ΣCT
      inclusive_wt_ms_threshold: ">=X"     # temps inclusif du nœud en ms
      exclusive_wt_ms_threshold: ">=X"     # temps propre (hors enfants) en ms
      time_share_percent_threshold: ">=X"  # part du wall time total, 0–100
    recommendation: <string, conseils de fix>
```

Deux périmètres d'observation : **par site d'appel** (`function_pattern`,
`call_count_threshold` — heuristique « in-loop ») et **par callee agrégé**
(seuils mémoire, temps, fan-out). Les critères définis se combinent en ET ;
au moins un critère positif est requis (`exclude_pattern` seul ne suffit pas).
Les unités des seuils sont celles de l'usage (ms, %, Mo) : la conversion
depuis les µs/octets internes se fait dans le moteur.

## Types clés à créer

- `Engine` — orchestre le chargement + le matching
- `Rule` — structure Go mirror du YAML
- `Matcher` — logique de matching (pattern, contexte, seuils)
- `Finding` — résultat d’un match (référence la Rule + localisation dans le graphe)

## Ce qu’il ne faut PAS

- Aucune logique de scoring → `scorer/`.
- Toute validation de schema dans le code → utilise le schéma JSON dans `proto/`.

## Tests

- Chargement + parsing de YAML (table-driven).
- Matching avec call graphs factices (fixtures dans `scripts/fixtures/`).
- Objectifs couverture/mutation : cf. racine `AGENTS.md` §5 (packages métier).

## Références croisées

- Entrée : `internal/analyzer/` (CallGraph)
- Schéma : `proto/` (rules.example.yaml, rules.schema.json)
- Sortie : `internal/scorer/` (score les findings)
