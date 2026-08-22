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
  - id: <string, unique>
    name: <string>
    description: <string>
    severity: critical|high|medium|low
    effort: low|medium|high
    controllability: controllable|partial|none
    match:
      function_pattern: <regex>
      ...
    recommendation: <string, conseils de fix>
```

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
