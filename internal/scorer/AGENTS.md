# AGENTS.md — internal/scorer/

> Instructions spécifiques à `internal/scorer/` — à lire **en plus** de la racine.

## Responsabilité

**Calculer le score de priorité** pour chaque Finding.

Formule : `Priority = Impact × (1/Effort) × Controllability`

- **Impact** : `% du temps total` × `nb d’appels`
- **Effort** : pondéré (low=1, medium=2, high=3) → inversé
- **Controllability** : `controllable=1.0`, `partial=0.5`, `none=0.2`
- Poids **ajustables** via config (yaml ou flags).

## Types clés à créer

- `Scorer` — calcule les scores à partir des Findings
- `PriorityScore` — struct Score, FindingID, Breakdown (impact/effort/controle)
- `Config` — poids configurables pour le scoring

## Ce qu’il ne faut PAS

- Aucune logique de matching → `rules/`.
- Aucune persistance directe → `storage/` (via retour de Score).

## Tests

- Tests unitaires table-driven sur la formule de scoring.
- Vérifier les seuils (ex : priority > 80 → critique).
- Objectifs couverture/mutation : cf. racine `AGENTS.md` §5 (packages métier).

## Références croisées

- Entrée : `internal/rules/` (Findings)
- Sortie : `internal/storage/` (persisté), `cmd/phperf-ci/` (comparaison baseline), `internal/ui/` (affichage)
