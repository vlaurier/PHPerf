# AGENTS.md — cmd/phperf/

> Instructions spécifiques à `cmd/phperf/` — à lire **en plus** de la racine.

## Responsabilité

**Thin wrapper** : point d'entrée du **binaire web UI**. Flags, wiring du
pipeline (collector → analyzer → rules → scorer), ouverture SQLite,
démarrage HTTP. Aucune logique métier.

## Implémentation actuelle

- Flags : `--profile --rules [--scoring] [--db=.phperf.db] [--addr=:8080]`.
- `run()` retourne ses erreurs au lieu de `log.Fatal` en ligne : les
  `defer` (fermeture de la base) doivent pouvoir s'exécuter (`gocritic
  exitAfterDefer`).
- `evaluate()` — même wiring que `cmd/phperf-ci/pipeline.go`. **Duplication
  volontaire** : deux binaires indépendants, pas de package partagé entre
  `cmd/*`.
- `toView()` — conversion `scorer.Scored` → vues `report.Finding`
  (l'ordre de priorité décroissante de `Score` est conservé).

## Ce qu'il ne faut PAS

- AUCUN code métier ici → tout va dans `internal/`.

## Références croisées

- Consomme : `internal/ui/`, `internal/report/`, `internal/storage/`,
  `internal/rules/`, `internal/scorer/`, `internal/analyzer/`,
  `internal/collector/`.
- Service compose `web` : lance le binaire sur la fixture de démo
  (`scripts/fixtures/nplus1.json`), port `${PHPERF_PORT:-8080}`.
