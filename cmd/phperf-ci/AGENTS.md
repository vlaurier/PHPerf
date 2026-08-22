# AGENTS.md — cmd/phperf-ci/

> Instructions spécifiques à `cmd/phperf-ci/` — à lire **en plus** de la racine.

## Responsabilité

**Thin wrapper** : point d’entrée du **binaire CI** (baseline).
→ Wiring DI + CLI flags + orchestration des étapes → `os.Exit(1)` uniquement
si des findings nouveaux n’apparaissent pas dans la baseline.

## Commandes (cobra)

- `phperf-ci baseline --profile=<json> --rules=<yaml>` — régénère
  **intégralement** la baseline (façon `phpstan -b`, pas de fusion).
- `phperf-ci run --profile=<json> --rules=<yaml> [--scoring=<yaml>]`
  `[--baseline=.phperf-baseline.json]` — compare et échoue sur les nouveaux.

Pipeline (wiring uniquement, un helper par étage dans `pipeline.go`) :
lecture du profil JSON → `collector.DecodeRaw` → `analyzer.Normalize` →
`rules.Load`+`Engine.Evaluate` → `baseline.Diff` (+ scorer pour l’affichage).

## Exit codes

| Code | Signification |
|---|---|
| 0 | Aucun nouveau finding par rapport à la baseline |
| 1 | Nouveaux findings non couverts par la baseline |
| 2 | Erreur d’exécution ou d’usage (fichiers, configs invalides…) |

## Ce qu’il ne faut PAS

- AUCUN code métier → tout va dans `internal/`.
- Aucun calcul de score/matching direct → délègue.

## Références croisées

- Consomme : `internal/collector/`, `internal/analyzer/`, `internal/rules/`,
  `internal/scorer/`, `internal/baseline/`.
- Format de clé : `<RuleID>|<callee>` — cf. jalon 2 (`docs/jalons.md`) ;
  limite acceptée : un renommage de méthode produit un « nouveau » finding.
- Sortie : CI (GitHub Actions, GitLab CI, etc.) ; la collecte PHP réelle
  alimentera `--profile` au jalon suivant.
