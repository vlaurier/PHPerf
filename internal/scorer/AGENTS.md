# AGENTS.md — internal/scorer/

> Instructions spécifiques à `internal/scorer/` — à lire **en plus** de la racine.

## Responsabilité

**Calculer le score de priorité** (0–100) pour chaque Finding.

Formule v1 : `Priority = 100 × TimeShare × poids_effort × poids_contrôlabilité`

- **TimeShare** : **mesuré** — part du wall time inclusif du callee dans la
  trace (`rules.Evidence.TimeShare`) ; aucun coefficient ne lui est appliqué.
- **Effort** / **Contrôlabilité** : **déclarés** par l'auteur de règles ;
  défauts {1.0 / 0.75 / 0.5} — modulation modérée, le temps reste le
  premier facteur de tri.
- `severity` hors formule (affichage seul) : l'inclure compterait deux fois
  une opinion face à la donnée mesurée.
- Poids **ajustables** via `proto/scoring.example.yaml` ; défauts embarqués
  (`DefaultWeights`).

## Types clés

- `Scorer` — calcule les scores à partir des Findings
- `Scored` — finding + priorité ; tri décroissant stable
- `WeightSet` + `LoadWeights` — pondérations validées (fichier ou défauts)

## Ce qu’il ne faut PAS

- Aucune logique de matching → `rules/`.
- Aucune persistance directe → `storage/`.

## Tests

- Tests unitaires table-driven sur la formule et le tri (stabilité incluse).
- Config : fusion sur défauts, clés inconnues/valeurs ≤ 0 rejetées.
- Objectifs couverture/mutation : cf. racine `AGENTS.md` §5 (packages métier).

## Références croisées

- Entrée : `internal/rules/` (Findings avec `Evidence.TimeShare`)
- Sortie : `cmd/phperf-ci/` (affichage), `internal/ui/`, `internal/report/`
  (jalon 5), `internal/storage/` (persisté, jalon 5)
