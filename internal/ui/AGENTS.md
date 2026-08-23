# AGENTS.md — internal/ui/

> Instructions spécifiques à `internal/ui/` — à lire **en plus** de la racine.

## Responsabilité

**Serveur HTTP de triage** : liste des findings priorisés (une page),
masquage/démasquage persisté via le `Store`. Zéro logique métier : les
findings arrivent déjà scorés et convertis en vues `report.Finding` par le
wiring de `cmd/phperf`.

## Implémentation actuelle

- `Store` — interface **définie côté consommateur** (`AddMask`,
  `RemoveMask`, `MaskedKeys`) ; implémentée par `*storage.DB` sans que ce
  package ne l'importe.
- `Server` — titre + liste de référence (ordre de priorité conservé) + store ;
  construit par `NewServer`, exposé via `Handler()`.
- Routes : `GET /` (liste ; `?show_masked=1` pour voir les masqués),
  `POST /mask` et `POST /unmask` (form `key=<clé stable>` puis redirect 303).
- Le rendu est délégué à `report.RenderHTML` — pas de template ici.

## Ce qu'il ne faut PAS

- Aucun import des couches métier (`collector`, `analyzer`, `rules`,
  `scorer`) ni de `storage` en direct : uniquement `report` + l'interface
  `Store` locale (cf. matrice racine).
- Aucune logique de filtrage/scoring au-delà du masquage affichage.

## Tests

- `httptest` + store en mémoire (`memStore`) et store en échec (`errStore`)
  pour les chemins 500 ; validation des méthodes HTTP (405) et de la clé
  requise (400).

## Références croisées

- Point d'entrée : `cmd/phperf/`.
- Rendu : `internal/report/` ; persistance : `internal/storage/`
  (via l'interface, jamais en direct).
