# AGENTS.md — scripts/

> Instructions spécifiques à `scripts/` — à lire **en plus** de la racine.

## Responsabilité

**Scripts utilitaires** pour le développement, les tests et l'intégration :

- Scripts PHP pour **générer des profils XHProf factices** (fixtures).
- Scripts PHP d'**intégration livrés aux utilisateurs**
  (`scripts/php/phperf-profile.php`, paire prepend/append HTTP) : colle
  générique de collecte — zéro connaissance applicative, aucune dépendance.
- Scripts shell pour **build**, **setup** (installer XHProf, etc.).
- `e2e.sh` : **test E2E de la chaîne de collecte** (côté hôte, opt-in
  `make e2e` — image php+xhprof → wrapper → assertions JSON → baseline/run
  → UI). POSIX-compatible ; hors quality gates.
- Scripts Go pour **seed la DB** de dev (`scripts/seed/`).

## Conventions

- PHP (fixtures & intégration) : syntaxe PHP 8.x, `declare(strict_types=1)`,
  vanilla (aucune dépendance Composer) ; les fichiers d'intégration doivent
  rester autonomes et copiables tels quels chez l'utilisateur.
- Shell : POSIX-compatible.
- Go (seed) : `go run` standalone, **hors du module principal** (ou
  `scripts/seed/` avec son propre `go.mod`).

## Ce qu'il ne faut PAS

- Aucun code de production Go ici → c'est `internal/` uniquement.
- Scripts dans `scripts/` ne doivent pas contenir de logique métier
  réutilisable (la normalisation XHProf dupliquée entre wrapper CLI et
  append HTTP est volontaire : fichiers autonomes exigés).

## Fixtures

- Place les sorties XHProf exemplaires dans `scripts/fixtures/` (format `.xhprof`).
- Utilisées par les tests de `internal/analyzer/` et `internal/rules/`.
