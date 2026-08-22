# AGENTS.md — internal/analyzer/

> Instructions spécifiques à `internal/analyzer/` — à lire **en plus** de la racine.

## Responsabilité

**Normaliser** les profils bruts (XHProf) en un **call graph unifié**.

- Parser le format XHProf (tableau `function==>caller` avec ct, wt, cpu, mu).
- Construire une structure de graphe d’appels (nodes = fonctions/classes, edges = appels).
- Produire un `CallGraph` normalisé consommé par `rules/` et `scorer/`.

## Types clés à créer

- `CallGraph` — graphe normalisé
- `Node` — fonction ou méthode (nom, classe, type d’entité)
- `Edge` — appel parent → enfant (nb appels, temps inclusif/exclusif)
- `Normalizer` — interface (implémentation `XHProfNormalizer`)

## Ce qu’il ne faut PAS

- Aucune logique de règles ou de scoring → `rules/` et `scorer/`.
- Aucune persistance → `storage/`.

## Tests

- Table-driven tests avec `testify`.
- Fixtures : échantillons de sortie XHProf dans `scripts/fixtures/`.
- Objectifs couverture/mutation : cf. racine `AGENTS.md` §5 (packages métier).

## TODO(phperf) — question ouverte

Pour la comparaison baseline en CI (`cmd/phperf-ci/`), les findings doivent
être identifiés de façon stable entre deux exécutions. La localisation portée
par le call graph doit donc être **déterministe** (même entrée → même clé) ;
le format exact de la clé reste à définir avec `cmd/phperf-ci/`.

## Références croisées

- Entrée : `internal/collector/` (RawProfile)
- Sortie : `internal/rules/` (matching contre le call graph), `internal/scorer/` (scoring)
