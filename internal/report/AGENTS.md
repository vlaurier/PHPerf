# AGENTS.md — internal/report/

> Instructions spécifiques à `internal/report/` — à lire **en plus** de la racine.

## Responsabilité

**Générer des rapports exportables** (HTML, JSON) à partir des findings
et scores — pour l’UI, la CI, ou le partage.

## Sorties attendues

- `HTMLReport` — page web autonome (findings, scores, graphe call, masquage)
- `JSONReport` — format machine (par ex. pour ingestion CI/CD, GitHub Checks)
- `ConsoleReport` — sortie texte pour `phperf-ci` (résultats visibles en logs CI)

## Types clés à créer

- `Generator` — interface (`Generate(ctx, findings, scores) ([]byte, error)`)
- `HTMLGenerator`, `JSONGenerator`, `ConsoleGenerator`

## Ce qu’il ne faut PAS

- Aucune logique de scoring ou matching — consomme des données déjà calculées.
- Aucune persistance — lecture via `storage/` si besoin.

## Références croisées

- Entrée : findings/scores déjà calculés — passés en paramètres ou relus
  depuis `internal/storage/`. **Pas d’import direct** des couches métier
  (`collector`, `analyzer`, `rules`, `scorer`) — cf. matrice racine.
- Sortie : `cmd/phperf-ci/` (console), `internal/ui/` (HTML)
