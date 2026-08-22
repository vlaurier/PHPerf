# Jalons & état d'implémentation — PHPerf

> Document de continuité : destiné à l'IA (reprise entre sessions Opencode)
> et aux contributeurs. **Mettre à jour à chaque jalon terminé.**
>
> - Vision produit & conventions : `AGENTS.md` (racine) + `AGENTS.md` par dossier.
> - Historique des décisions fondatrices : `brainstorming.txt` (archive, ne pas éditer).
> - Architecture cible : `docs/architecture.md`.

---

## Résumé exécuté / restant (état au 22 août 2026)

| Jalon | Contenu | Statut |
|---|---|---|
| 0 | Environnement dev (Docker, Makefile, lint v2, docs pivot baseline) | ✅ Fait |
| 1 | Contrat collecteur (`collector.RawProfile`) + normalisation XHProf (`analyzer`) | ✅ Fait |
| 2 | Moteur de règles YAML (`internal/rules`) | ✅ Fait |
| 3 | Scoreur de priorité (`internal/scorer`) | ✅ Fait |
| 4 | Storage SQLite + baseline CI (`storage`, `phperf-ci`) | ⬜ À faire |
| 5 | Rapports & UI web (`report`, `ui`, service `web`) | ⬜ À faire |

Qualité : `make check` vert sur les jalons 0-3 ; couverture **100 %**
constatée sur les quatre packages métier (`collector`, `analyzer`,
`rules`, `scorer` — §5 racine) ; complexité ≤ 15 vérifiée par `cyclop`.

Repo git initialisé le 22/08/2026 (branche `main`, premier commit `f5f653a`).

---

## Jalon 0 — Environnement & outillage (FAIT)

- **Dockerfile** : image dev `golang:1.27-bookworm`, `golangci-lint` version
  épinglée. Rien à installer sur l'hôte, seul Docker est requis.
- **docker-compose.yml** : deux services — `app` (outils : go, lint, tests,
  build) et `web` (future UI, port 8080, variable `PHPERF_PORT`).
- **Makefile** (tout passe par le conteneur) :
  `help fix lint vet tidy tests check build shell up down logs clean`.
  Commandes clés : `make fix` (fmt + auto-fix), `make check` (lint + vet +
  tests race+cover — doit être vert avant tout commit).
- **`.golangci.yml` (format v2)** :
  - linters : errcheck, goconst (`ignore-tests: true`), gocritic, govet,
    ineffassign, revive, staticcheck (englobe gosimple), unparam, cyclop
    (`max-complexity: 15`) ;
  - **misspell volontairement absent** : il ne connaît que l'anglais et son
    `--fix` a corrompu des commentaires français (voir « Leçons » ci-dessous) ;
  - **depguard = matrice de dépendances** (miroir de `AGENTS.md` §3) :
    chaque package interne liste ce qui lui est interdit d'importer. Toute
    nouvelle dépendance impose de modifier la matrice ici ET dans AGENTS.md ;
  - formatters : gofmt (simplify), goimports (`local-prefixes
    github.com/phperf/phperf`), gofumpt.
- **Docs pivotées** : suppression de toutes les mentions Blackfire ;
  la CI ne repose plus sur des « quality gates à seuil » mais sur une
  **baseline façon PHPStan** (`phperf-ci` échoue uniquement sur les findings
  nouveaux, jamais sur l'existant). Propagé dans README, AGENTS.md racine,
  `docs/architecture.md`, AGENTS.md de `cmd/phperf-ci`, `internal/{rules,
  storage,scorer,report}`, `proto/`.

## Jalon 1 — Contrat collecteur + normalisation XHProf (FAIT)

### `internal/collector/profile.go` (contrat de données seulement)

- `RawProfile` — `map[string]Entry` : profil brut XHProf sérialisé en JSON.
  Clés : `"main()"` (racine) ou `"parent==>enfant"`.
- `Entry` — `{CT, WT, CPU, MU, PMU}` (tags JSON `ct/wt/cpu/mu/pmu`,
  µs pour les temps, octets pour la mémoire).
- `DecodeRaw(data []byte)` — décode le JSON canonique ; erreur si illisible
  ou vide.
- La collecte réelle (exécution de PHP + pont JSON) viendra plus tard —
  voir jalon 4/5, questions ouvertes Q2.

### `internal/analyzer/`

- `graph.go` :
  - `Node` — `{Name, IsRoot, CallCount, InclusiveWT, ExclusiveWT}` ;
  - `Edge` — appel parent→enfant avec coût via ce parent
    `{Caller, Callee, CT, WT, CPU, MU}` + méthode `key()` (`caller==>callee`) ;
  - `CallGraph` — `{Root *Node, Nodes map[string]*Node, Edges []Edge,
    Children map[string][]Edge}` (Children = index des arêtes sortantes).
- `xhprof.go` (~215 lignes) :
  - `RootName = "main()"`, séparateur `"==>"` ;
  - interface `Normalizer` + implémentation `XHProfNormalizer` ;
  - erreurs sentinelles : `ErrMissingRoot`, `ErrNegativeMetrics`,
    `ErrInvalidEntry` (clé sans `caller==>callee`), `ErrUnreachableNode` ;
  - `Normalize(raw)` : valide racine + chaque entrée (métriques ≥ 0),
    construit le graphe, calcule les métriques, trie `Edges` par clé pour un
    ordre **déterministe** (exigence baseline, cf. TODO `analyzer/AGENTS.md`).

#### Algorithme des métriques (point délicat — ne pas régresser)

1. `addEdge` ne cumule **que** `CallCount`. Tout le calcul des temps est
   fait ensuite par `computeMetrics`.
2. `findRecursiveEdges` : DFS depuis la racine avec pile d'appels courante.
   Une arête dont la cible est **déjà dans la pile** est marquée
   « récursive » : le coût qu'elle porte est déjà contenu dans le temps du
   parent qui referme le cycle.
3. `inclusif(nœud) = Σ arêtes entrantes non récursives` (la racine garde sa
   valeur propre du profil) ;
   `exclusif(nœud) = inclusif − Σ arêtes sortantes non récursives`.
4. Tout nœud non atteignable depuis la racine → rejet
   `ErrUnreachableNode` (données malformées, métriques trompeuses).

Bug corrigé en fin de jalon : la v1 cumulait `InclusiveWT` dans `addEdge`
et soustrayait en DFS → sur un profil récursif type fib, l'inclusif était
doublé (3100 au lieu de 1900). Les tests de non-régression sont en place.

### Tests & fixtures

- `xhprof_test.go` (table-driven, testify) : table d'erreurs (profil nil/vide,
  racine sans appel, entrée isolée, métriques négatives, nœud injoignable),
  profil linéaire, diamant multi-parents (somme inclusive partagée),
  récursion `fib` (pas de double comptage), tri déterministe des arêtes,
  intégration : tous les `.json` de `scripts/fixtures/` sont normalisables.
- `scripts/fixtures/` : `linear.json`, `nplus1.json`, `recursive.json`.

### Préparé pour les jalons suivants (non codé)

- `proto/rules.example.yaml` — 5 règles types (network-call-in-loop,
  duplicated-calculation, n-plus-one-query, deep-recursion,
  memory-heavy-allocation) + `proto/rules.schema.json`.
- `cmd/phperf` et `cmd/phperf-ci` — stubs (« à implémenter »), zéro logique.

---

## Jalons suivants (proposition, à valider avant de coder)

### Jalon 2 — Moteur de règles YAML (`internal/rules`) — FAIT

Décisions validées avec le pilote (22/08/2026) :

- **Clé stable baseline** (Q1 résolue) : `<RuleID>|<callee>`. Un renommage de
  méthode produit un « nouveau » finding — limite assumée, comme les chemins
  de PHPStan.
- **Règles « *-in-loop »** (Q3 résolue) : heuristique retenue — match si le
  CT d'une arête caller→callee ≥ N. Aucun profileur ne portera jamais
  l'info boucle (une boucle n'est pas une fonction) ; l'heuristique est
  précise en pratique et la recommandation reste pertinente.
- **duplicated-calculation** : gardée, matcher affaibli (CT seul). Nos
  backends prévus (XHProf, php-spx, phpspy) n'exposeront pas les arguments ;
  Xdebug `collect_params` est hors périmètre CI (overhead).
- **deep-recursion** : écartée — dériver la profondeur du CT serait une
  approximation. Réintégration possible via un futur backend à traces.
- Seuils au format `">=N"` uniquement en v1 (`Threshold` custom YAML).

Livré :

- `rule.go` — types du format (`Rule`, `Match`, enums typés, type
  `Threshold` avec `UnmarshalYAML` strict) ;
- `loader.go` — `Load(data)` strict : champs inconnus rejetés
  (`KnownFields`), id unique `^[a-z0-9-]+$`, name/description requis,
  enums valides, ≥ 1 critère, regexp compilable ;
- `finding.go` — `Finding{Key, RuleID, Function, Caller, Severity, Effort,
  Controllability, Recommendation, Evidence}` + `Evidence{CallCount,
  MemPerCallMB}` ;
- `engine.go` — interface `Evaluator`, `Engine.Evaluate` :
  function_pattern sur le callee (racine exclue), call_count **par arête**,
  mémoire par appel **par callee** (ΣMU/ΣCT), findings dédoublonnés par clé
  stable, `Caller` = site dominant (max CT), résultat déterministe ;
- `proto/rules.example.yaml` réécrit (4 règles honnêtes) et
  `rules.schema.json` aligné sur le format v1 ;
- tests table-driven (chargement : 15 cas d'erreur inclus ; moteur : pattern,
  périmètre par arête, dédoublonnage dominant, mémoire agrégée, ordre,
  racine exclue) + intégration exemple×fixture `nplus1.json`.

### Jalon 3 — Scoreur (`internal/scorer`) — FAIT

Décisions validées avec le pilote (22/08/2026) :

- **Score normalisé 0–100** (vs formule brute non bornée du brainstorming
  §4.2).
- **Pas de multiplicateur par nombre d'appels** : le temps inclusif du callee
  agrège déjà ses répétitions (50 requêtes N+1 pèsent dans son WT) ;
  multiplier encore par CT reviendrait à compter deux fois le même coût.
  CT reste dans `Evidence` pour affichage/filtrage CI ultérieur.
- **Le scoreur ne consomme que des findings** (matrice depguard respectée :
  scorer → rules uniquement) : `rules.Evidence` a été enrichi de
  `TimeShare` (part du wall time du callee dans la trace), calculé par
  l'engine qui a accès au graphe.
- **Config YAML ajustable dès ce jalon** (l'originalité produit) :
  `proto/scoring.example.yaml` + `scoring.schema.json` ; fusion sur les
  défauts embarqués, décodage strict comme pour les règles.

Formule v1 :

    Priority = 100 × TimeShare × poids_effort × poids_contrôlabilité

Défauts : effort low=1.0 / medium=0.6 / high=0.3 ; contrôlabilité
controllable=1.0 / partial=0.6 / none=0.3.

Livré :

- `score.go` — interface `Scorer`, `Engine.Score` (tri décroissant stable,
  priorité nulle si l'enum d'un finding est absent des pondérations),
  `DefaultWeights`, validation à la construction (poids > 0) ;
- `config.go` — `LoadWeights(data)` strict : champs inconnus rejetés,
  clés d'enum contrôlées, valeurs > 0, fusion sur les défauts ;
- `proto/scoring.example.yaml` + `proto/scoring.schema.json` ;
- tests table-driven (formule, tri stable, part de temps nulle, enum
  inconnu, pondérations invalides, 8 configs rejetées, exemple = défauts).

### Jalon 4 — Storage SQLite + baseline CI (`storage`, `cmd/phperf-ci`)

- Schéma SQLite : profils, findings, décisions de masquage (persistant).
- Baseline façon PHPStan : `phperf-ci baseline` génère/met à jour
  `.phperf-baseline.json` ; `phperf-ci run --baseline=…` compare et
  `exit(1)` uniquement sur findings nouveaux (codes 0/1/2 documentés dans
  `cmd/phperf-ci/AGENTS.md`).
- Collecte réelle : orchestration PHP + XHProf + pont JSON
  (`perftools/php-profiler`), scripts dans `scripts/`.

### Jalon 5 — Rapports & UI web (`report`, `ui`, service `web`)

- Rapport HTML/JSON (`internal/report`), UI minimaliste : liste des
  findings, scores, suggestions, boutons de masquage ; servi par
  `cmd/phperf` (service compose `web`, port 8080).

Post-MVP : backends php-spx / phpspy, règles communautaires importables,
suggestions LLM.

---

## Questions ouvertes (TODO(phperf))

1. Format exact du pont PHP→Go (JSON intermédiaire) et modalités de
   collecte (script dédié ? intégration request HTTP ?) — impactera
   `internal/collector`, à trancher au jalon 4.

## Leçons & conventions acquises en cours de route

- **misspell a corrompu des commentaires français** via `make fix`
  (« respecte »→« respective », « transforme »→« transfer », « normalisé »→
  « normalsé ») → linter retiré ; commentaires réparés. Si une coquille
  bizarre apparaît dans un commentaire, suspecter un reliquat.
- **goconst** : le réglage `goconst.ignore-tests: true` fonctionne en v2 ;
  ne pas passer par `exclusions.rules` pour ça.
- **Workflow convenu avec le pilote** : proposer les commandes et attendre
  validation avant de les exécuter ; si une sortie d'erreur semble
  incohérente, demander une sortie fraîche (un copier/coller obsolète a déjà
  induit un mauvais diagnostic).
- Toujours relire un fichier après un passage de `golangci-lint fmt` avant
  d'éditer (le formatage peut avoir décalé les lignes).

## Reprendre le travail plus tard

- Sessions Opencode : `opencode --continue` (dernière session) ou
  `opencode session list` puis `opencode --session <id>` ; export possible
  via `opencode export`.
- Sinon : coller ce fichier (`docs/jalons.md`) dans une nouvelle session —
  il résume l'état complet. Pointer également vers les `AGENTS.md` (lus
  automatiquement).
