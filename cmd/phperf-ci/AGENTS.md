# AGENTS.md — cmd/phperf-ci/

> Instructions spécifiques à `cmd/phperf-ci/` — à lire **en plus** de la racine.

## Responsabilité

**Thin wrapper** : point d’entrée du **binaire CI** (baseline).
→ Wiring DI + CLI flags + orchestration des étapes → `os.Exit(1)` uniquement
si des findings nouveaux n’apparaissent pas dans la baseline.

## Pipeline exécuté (ou commandes cobra)

1. `Collect` (via `internal/collector/`)
2. `Analyze` (via `internal/analyzer/`)
3. `EvaluateRules` (via `internal/rules/`)
4. `Score` (via `internal/scorer/`)
5. `CompareBaseline` — filtrer les findings déjà connus, identifier les nouveaux
6. `os.Exit(0)` si aucun nouveau finding, `os.Exit(1)` sinon, `os.Exit(2)` si erreur runtime

Commandes prévues :

- `phperf-ci baseline` — génère / met à jour la baseline
- `phperf-ci run --baseline=.phperf-baseline.json` — exécution + comparaison

## Exit codes

| Code | Signification |
|---|---|
| 0 | Aucun nouveau finding par rapport à la baseline |
| 1 | Nouveaux findings non couverts par la baseline |
| 2 | Erreur d’exécution (profilage impossible, etc.) |

## TODO(phperf) — question ouverte

La comparaison à la baseline suppose qu'un finding soit identifiable de façon
**stable entre deux exécutions** (ex : Rule ID + fonction localisée). Le
format de cette clé — et sa résistance aux renommages/refactorings — reste à
définir avec `internal/analyzer/` et `internal/storage/`.

## Ce qu’il ne faut PAS

- AUCUN code métier → tout va dans `internal/`.
- Aucun calcul de score/matching direct → délègre.

## Références croisées

- Consomme : `internal/collector/`, `internal/analyzer/`, `internal/rules/`, `internal/scorer/`, `internal/report/`.
- Sortie : CI (GitHub Actions, GitLab CI, etc.)
