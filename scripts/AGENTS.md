# AGENTS.md — scripts/

> Instructions spécifiques à `scripts/` — à lire **en plus** de la racine.

## Responsabilité

**Scripts utilitaires** pour le développement et les tests :

- Scripts PHP pour **générer des profils XHProf factices** (fixtures).
- Scripts shell pour **build**, **setup** (installer XHProf, etc.).
- Scripts Go pour **seed la DB** de dev (`scripts/seed/`).

## Conventions

- PHP (fixtures) : syntaxe PHP 8.x.
- Shell : POSIX-compatible.
- Go (seed) : `go run` standalone, **hors du module principal** (ou `scripts/seed/` avec son propre `go.mod`).

## Ce qu’il ne faut PAS

- Aucun code de production ici → c’est `internal/` uniquement.
- Scripts dans `scripts/` ne doivent pas contenir de logique métier réutilisable.

## Fixtures

- Place les sorties XHProf exemplaires dans `scripts/fixtures/` (format `.xhprof`).
- Utilisées par les tests de `internal/analyzer/` et `internal/rules/`.
