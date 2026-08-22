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
| 2 | Moteur de règles YAML (`internal/rules`) | ⬜ À faire |
| 3 | Scoreur de priorité (`internal/scorer`) | ⬜ À faire |
| 4 | Storage SQLite + baseline CI (`storage`, `phperf-ci`) | ⬜ À faire |
| 5 | Rapports & UI web (`report`, `ui`, service `web`) | ⬜ À faire |

Qualité actuelle : `make check` vert ; couverture **100 %** sur `collector`
et `analyzer` ; complexité ≤ 15 vérifiée par `cyclop`.

⚠️ **Le dépôt n'est pas encore un repo git** (`git init` non fait) — aucune
historisation des changements. À traiter avant le jalon 2 idéalement.

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

### Jalon 2 — Moteur de règles YAML (`internal/rules`)

- Loader `yaml.v3` validé contre `rules.schema.json` (ou struct tags strictes).
- Type `Finding` : `{RuleID, Node/localisation, Severity, Effort,
  Controllability, Recommendation}` — **la localisation doit être
  déterministe** (même entrée → même clé), cf. Q1.
- Matchers v1 réalistes sur ce que le call graph sait faire :
  `function_pattern`, `call_count_threshold`, ratios mémoire/appel…
- ⚠️ Limite connue : le format XHProf **ne porte pas l'information « dans
  une boucle »**. Les règles `*-in-loop` (network-call-in-loop, n+1)
  nécessitent soit une heuristique (N appels identiques depuis un même
  caller), soit un enrichissement de la collecte — cf. Q3.

### Jalon 3 — Scoreur (`internal/scorer`)

- Formule brainstorming §4.2 : `Impact` (% wall × nb appels) × `Effort`
  × `Contrôlabilité` ; poids ajustables ; sortie = findings priorisés.

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

1. **Clé stable des findings** pour la comparaison baseline (résistance aux
   renommages/refactorings) — à définir ensemble, impacte `analyzer`,
   `storage`, `phperf-ci`. Noté dans `analyzer/AGENTS.md` et
   `cmd/phperf-ci/AGENTS.md`.
2. Format exact du pont PHP→Go (JSON intermédiaire) et modalités de
   collecte (script dédié ? intégration request HTTP ?).
3. Détection des boucles : hors périmètre XHProf → heuristique ou
   enrichissement collecte (cf. jalon 2).
4. `git init` + premier commit toujours pas faits.

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
