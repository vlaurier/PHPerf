# AGENTS.md — cmd/phperf/

> Instructions spécifiques à `cmd/phperf/` — à lire **en plus** de la racine.

## Responsabilité

**Thin wrapper** : point d’entrée du **binaire web UI**.
→ Wiring DI + `http.ListenAndServe`. **Aucune logique métier.**

## Structure attendue

```go
package main

func main() {
    // 1. Charger config (.phperf.yaml ou flags)
    // 2. Bootstrap DI (storage, ui.Server, rules.Engine, scorer.Scorer)
    // 3. http.ListenAndServe(...)
}
```

## Ce qu’il ne faut PAS

- AUCUN code métier ici → tout va dans `internal/`.
- Aucun matching, scoring, collection → délègre.

## Références croisées

- Consomme : `internal/ui/`, `internal/storage/`, `internal/rules/`, `internal/scorer/`, `internal/analyzer/`.
