# AGENTS.md — internal/collector/

> Instructions spécifiques à `internal/collector/` — à lire **en plus** de la racine.

## Responsabilité

Orchestrer la **collecte de données de profilage** depuis une application PHP cible.

- Démarrer PHP avec le backing store activé (XHProf par défaut).
- Exécuter une requête cible (script PHP, URL HTTP, etc.).
- Capturer la sortie brute du profilage et la convertir en format interne.
- Gérer la **pluggable architecture** des backends (XHProf, php-spx, phpspy — futur).

## Types clés à créer

- `Collector` — interface (méthode `Collect(ctx) (*RawProfile, error)`)
- `XHProfCollector` — implémentation concrète pour XHProf.
- `RawProfile` — format brut normalisé (à consommer par `analyzer/`).

## Ce qu’il ne faut PAS

- Aucune logique d’analyse ou de scoring → c’est le rôle de `analyzer/` et `scorer/`.
- Aucune persistance → c’est le rôle de `storage/`.

## Références croisées

- Entrée : `cmd/phperf-ci/` (orchestrateur CI)
- Sortie : `internal/analyzer/` (normalisation du call graph)
- Helpers : `scripts/` (scripts PHP pour générer des profils de test)
