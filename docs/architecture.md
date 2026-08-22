# Architecture — PHPerf

Ce document décrit le **flux de données** à travers les composants et les
responsabilités de chacun. Les contraintes de développement par dossier
(interdits, conventions, tests) sont détaillées dans les `AGENTS.md`
respectifs — ce document donne la vue d'ensemble.

## Vue d'ensemble du pipeline

```
[ App PHP cible (XHProf activé) ]
        |  profil brut (ct, wt, cpu, mu)
        v
[ internal/collector ]      Collector (interface) / XHProfCollector → RawProfile
        |  RawProfile
        v
[ internal/analyzer ]       XHProfNormalizer → CallGraph (Node / Edge)
        |  CallGraph
        v
[ internal/rules ]          moteur YAML → Findings + recommandations
        |  Findings
        v
[ internal/scorer ]         Priority = Impact × (1/Effort) × Contrôlabilité
        |  PriorityScores
        v
[ internal/storage ]        SQLite : Profile, Finding, PriorityScore, MaskedFinding
        |
        +--> [ internal/ui ]        interface web (exploration, triage)
        +--> [ internal/report ]    rapports HTML / JSON / console
        +--> [ cmd/phperf-ci ]      comparaison baseline : exit 0 / 1 / 2
```

Deux binaires (`cmd/`) orchestrent ce pipeline, sans logique métier :

- `phperf` — sert l'interface web (wiring DI + `http.ListenAndServe`).
- `phperf-ci` — exécute collecte → analyse → règles → scoring → comparaison
  à la baseline.

## Composants

### `internal/collector` — collecte

Orchestre le profilage de l'application cible : démarre PHP avec le backing
activé (XHProf par défaut), exécute la requête ou le script cible, capture la
sortie brute. Architecture **plugable** : `Collector` est une interface,
`XHProfCollector` la première implémentation (php-spx, phpspy prévus).

- Entrée : configuration d'exécution · Sortie : `RawProfile`.

### `internal/analyzer` — normalisation

Parse le format XHProf (`function==>caller` avec ct/wt/cpu/mu) et construit un
**call graph unifié** : nœuds = fonctions/méthodes, arêtes = appels (nombre
d'appels, temps inclusif/exclusif).

- Entrée : `RawProfile` · Sortie : `CallGraph`, consommé par `rules/` et `scorer/`.

### `internal/rules` — détection des anti-patterns

Moteur déclaratif : applique des règles YAML au call graph et produit des
`Finding`s avec recommandation de correctif. Le format est défini dans
[`proto/`](../proto/) (exemple commenté + schéma JSON) ; une règle porte :
`match` (pattern de fonction, contexte de boucle, seuils), `severity`,
`effort`, `controllability`, `recommendation`. Une règle ne décide jamais
de ce qui bloque en CI : c'est le rôle de la baseline.

Règles classiques du MVP : requête SQL en boucle (N+1), appel réseau en boucle,
calcul dupliqué, récursion profonde, allocation mémoire excessive.

### `internal/scorer` — priorisation

Calcule pour chaque finding un score de priorité :

```
Priority = Impact × (1/Effort) × Controllability
```

| Facteur | Valeurs |
|---|---|
| Impact | `% du temps total (wall)` × `nb d'appels` |
| Effort | low=1, medium=2, high=3 (inversé dans la formule) |
| Contrôlabilité | controllable=1.0, partial=0.5, none=0.2 |

Les poids sont **ajustables** via config (YAML ou flags) ; la priorité par
défaut reste automatique.

### `internal/storage` — persistance SQLite

Fichier unique (driver pure Go `modernc.org/sqlite`, pas de CGO — binaire CI
portable). Stocke profils, findings, scores et surtout les décisions de
triage. Migrations SQL dans `internal/storage/schema/`.

### `internal/ui` — interface web

Handlers HTTP (`net/http`) + templates Go natifs (`html/template`), sans
framework JS. Permet d'explorer les profils, consulter les findings priorisés,
masquer/démasquer et ajuster les scores.

### `internal/report` — rapports exportables

Génère des sorties à partir des findings/scores déjà calculés :
rapport HTML autonome, JSON machine (ingestion CI/CD), sortie console lisible
pour `phperf-ci`.

## Masquage & triage

Le masquage est la fonctionnalité différenciante côté workflow :

1. Un développeur masque un finding depuis l'UI (raison conservée).
2. `storage/` enregistre un `MaskedFinding` (Rule ID + fonction, pattern,
   raison, date).
3. En CI, `phperf-ci` ignore tout finding masqué correspondant au même pattern.
4. Le masquage est **global et persistant** : il survit aux exécutions
   futures jusqu'à démasquage explicite.

Ainsi un goulot assumé (ex : API externe déjà mise en cache ailleurs) ne fait
plus échouer indéfiniment la CI.

## Baseline CI

Sur le modèle de PHPStan, `phperf-ci` juge la **régression**, pas l'existant :

1. La première passe (`phperf-ci baseline`) enregistre les findings courants
   dans une baseline versionnée avec le projet (`.phperf-baseline.json`).
2. En CI (`phperf-ci run`), tout finding présent dans la baseline est
   considéré comme connu et ignoré.
3. L'exécution échoue uniquement si des findings nouveaux apparaissent.

| Exit code | Signification |
|---|---|
| `0` | Aucun nouveau finding par rapport à la baseline |
| `1` | Nouveaux findings non couverts par la baseline |
| `2` | Erreur d'exécution (profilage impossible…) |

Un échec produit aussi un rapport console/JSON exploitable dans les logs CI
(GitHub Actions, GitLab CI…). Le masquage (voir plus haut) complète la
baseline pour les goulots assumés durablement.

## Points d'extension

- **Backends de profilage plugables** : XHProf aujourd'hui, php-spx et
  phpspy (sampling, faible overhead) ensuite — via l'interface `Collector`.
- **Règles utilisateur & communautaires** : fichiers YAML importables,
  validés par le schéma JSON.
- **Multi-langage** (futur) : parser commun vers le call graph unifié.

## Roadmap MVP (phase 1)

1. Collecte XHProf (`perftools/php-profiler`) → JSON intermédiaire.
2. Parseur Go du call graph XHProf (`analyzer`).
3. Moteur de règles YAML avec ~5 règles classiques.
4. Scorer avec formule de priorité.
5. CLI `phperf-ci` : génération de baseline, échec uniquement sur les
   nouveaux findings.
6. UI web minimale : liste des findings, scores, masquage, suggestions.
