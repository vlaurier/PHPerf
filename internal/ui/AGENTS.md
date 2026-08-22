# AGENTS.md — internal/ui/

> Instructions spécifiques à `internal/ui/` — à lire **en plus** de la racine.

## Responsabilité

**Interface web** (HTTP handlers + templates) pour explorer les profils,
visualiser les findings priorisés, gérer le **masquage/triage**, et
ajuster les scores.

## Tech stack

- Go `net/http` standard (ou `spf13/cobra` pour routing léger).
- Templates HTML natifs Go (`html/template`) — **0 JS framework**.
- CSS minimal (Bootstrap CDN ou simple).

## Types clés à créer

- `Server` — HTTP server (configuration, DI)
- `Handler` — handlers pour : `/`, `/profiles`, `/findings`, `/mask`, `/rules`
- `TemplateRenderer` — wrapper autour de `html/template`

## Ce qu’il ne faut PAS

- Aucune logique de scoring/matching/analyse → délègue à `scorer/`, `rules/`, `storage/`.
- `cmd/phperf/` est le **thin wrapper** → wiring DI + `http.ListenAndServe` uniquement.

## Tests

- Tests HTTP avec `httptest` (table-driven).
- Vérifier les status codes + le masquage via l’UI.

## Références croisées

- Dépend de : `internal/storage/` (lecture) et `internal/report/` (rendu
  HTML). **Jamais** les couches métier directement (`collector`, `analyzer`,
  `rules`, `scorer`) — cf. matrice racine.
- Point d’entrée : `cmd/phperf/`.
