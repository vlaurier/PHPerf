# AGENTS.md — internal/storage/

> Instructions spécifiques à `internal/storage/` — à lire **en plus** de la racine.

## Responsabilité

**Persistance SQLite** : stocker les profils, findings, scores,
et les **décisions de masquage / tri** (triage).

- Schéma SQLite fichier-unique (`~/.phperf.db` ou `.phperf.sqlite`).
- Migrations SQL dans `internal/storage/schema/`.
- CRUD pour : `Profile`, `Finding`, `PriorityScore`, `MaskedFinding`.

## Persistance du masquage (triage)

- Quand un dev masque un finding → enregistrer `MaskedFinding` (FindingID, profile_pattern, reason, created_at).
- En CI (`phperf-ci`) → ignorer les findings masqués correspondant au
  même pattern (ex : même Rule ID + même fonction).
- Le masquage est **global** (toutes les exécutions futures).

## Types clés à créer

- `DB` — wrapper SQLite (géométrie simple, `database/sql` + driver)
- `Profile`, `Finding`, `PriorityScore`, `MaskedFinding` — modèles
- `Repository` interfaces pour chaque entité (facilite les mocks en tests)

## Driver SQLite recommandé

- `modernc.org/sqlite` (pure Go, pas de CGO) — idéal pour binaire CI portable.

## Tests

- Tests contre DB en mémoire (`:memory:`).
- Table-driven pour chaque opération CRUD.

## Références croisées

- Consommateur principal : `internal/ui/` (lecture masquage), `cmd/phperf-ci/` (comparaison baseline).
- Dépend de : `internal/analyzer/`, `internal/rules/`, `internal/scorer/` (types persistés).
