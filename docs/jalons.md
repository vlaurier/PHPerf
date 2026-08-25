# Jalons & état d'implémentation — PHPerf

> Document de continuité : destiné aux contributeurs et aux assistants IA
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
| 4 | Baseline CI (`baseline`, `phperf-ci`) — storage déplacé au 5 | ✅ Fait |
| 5 | Rapports & UI web (`report`, `ui`, `storage`, service `web`) | ✅ Fait |
| 6 | Expressivité moteur + catalogue de règles (20 règles testées) | ✅ Fait |
| 7 | Collecte PHP réelle (pont XHProf → JSON) | ✅ Fait |
| 8 | Distribution des binaires (release multi-OS) | ⬜ À faire |

Qualité : `make check` vert sur les jalons 0-8 ; couverture **100 %**
constatée sur les cinq packages métier (`collector`, `analyzer`, `rules`,
`scorer`, `baseline`) ; packages d'infrastructure entre 91 et 98 %
(`report`, `storage`, `ui` — branches restantes : erreurs driver/scan
inatteignables sans injection) ; complexité ≤ 15 vérifiée par `cyclop`.
Contrat d'exit codes de `phperf-ci` vérifié en conditions réelles (0 sur
baseline à jour, 1 sur nouveaux findings — démo `make ci-demo`).

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

Défauts (ajustés le 23/08/2026 avec le pilote) : effort low=1.0 / medium=0.75
/ high=0.5 ; contrôlabilité controllable=1.0 / partial=0.75 / none=0.5.
Modulation **modérée** (÷4 max combiné) : le temps mesuré doit rester le
premier facteur de tri ; effort et contrôlabilité — déclarés par l'auteur
de règles, jamais calculés — ne font que départager. La `severity` est
**hors formule** (affichage seul) : l'inclure compterait deux fois une
opinion face à la donnée mesurée.

Livré :

- `score.go` — interface `Scorer`, `Engine.Score` (tri décroissant stable,
  priorité nulle si l'enum d'un finding est absent des pondérations),
  `DefaultWeights`, validation à la construction (poids > 0) ;
- `config.go` — `LoadWeights(data)` strict : champs inconnus rejetés,
  clés d'enum contrôlées, valeurs > 0, fusion sur les défauts ;
- `proto/scoring.example.yaml` + `proto/scoring.schema.json` ;
- tests table-driven (formule, tri stable, part de temps nulle, enum
  inconnu, pondérations invalides, 8 configs rejetées, exemple = défauts).

### Jalon 4 — Baseline CI (`internal/baseline`, `cmd/phperf-ci`) — FAIT

Décisions validées avec le pilote (22/08/2026) :

- **Périmètre recentré** : la collecte PHP réelle et le stockage SQLite
  sortent du jalon. Rationale : la CI est **autonome sur le fichier**
  `.phperf-baseline.json` (reproductible partout, zéro état externe) ;
  le triage/masquage SQLite n'a de consommateur qu'à partir de l'UI
  (jalon 5) — pas de code sans user story (§7 racine). La collecte réelle
  nécessite un runtime PHP absent de l'image dev → jalon dédié ensuite.
- **`phperf-ci baseline`** : régénération complète façon `phpstan -b`
  (pas de fusion incrémentale).
- **Driver SQLite (reporté au jalon 5)** : `modernc.org/sqlite` pur Go,
  sans cgo.
- **Nouvelle couche depguard** : `baseline → rules` ; matrice
  `.golangci.yml` et AGENTS.md §3 mises à jour ensemble.

Livré :

- `internal/baseline/baseline.go` — format v1 `{version, entries[{key,
  rule_id, function}]}` ; `Load` strict (`DisallowUnknownFields`, version
  contrôlée) ; `Save` déterministe (tri par clé, aucun horodatage → diffs
  git stables d'une régénération à l'autre) ; `Diff(findings, baseline)`
  → nouveaux / connus ;
- `cmd/phperf-ci` (cobra) — `baseline` et `run --profile --rules
  [--scoring] [--baseline]`, exit codes 0/1/2, rapport texte listant les
  nouveaux findings (priorité + recommandation) ;
  pipeline = wiring collector → analyzer → rules (+ scorer affichage) dans
  `pipeline.go` ; zéro logique métier dans cmd ;
- dépendance ajoutée : `spf13/cobra` (stack prévue §2 racine) ;
- tests table-driven baseline (chargement strict, tri/déterminisme,
  round-trip, diff mixte/vide/tout-connu) ; cible `make ci-demo`
  (baseline puis run sur la fixture nplus1).

### Jalon 5 — Storage, rapports & UI web (`storage`, `report`, `ui`, `cmd/phperf`) — FAIT

Décisions validées avec le pilote (23/08/2026) :

- **UI d'abord** : le service `web` consomme la fixture de démo
  (`scripts/fixtures/nplus1.json`) ; la collecte PHP réelle devient un
  jalon dédié ensuite.
- **Profil chargé en mémoire au démarrage** ; SQLite ne persiste **que les
  masques** (clés stables traitées comme chaînes opaques). Le masquage est
  un état personnel local (`.phperf.db`, non versionné) — la CI reste
  autonome sur le fichier baseline ; un export masques→baseline pourra
  s'ajouter plus tard si besoin.
- **Report = DTO de vue propres** (`report.Finding` construits par le
  wiring `cmd/*`) : aucun import métier dans `report`/`ui`, la matrice
  depguard n'a pas bougé. Le rendu est passif : le filtrage des masqués
  appartient à l'UI (`?show_masked=1`).
- Driver SQLite : `modernc.org/sqlite` pur Go (décision du jalon 4).

Livré :

- `internal/storage/storage.go` — table unique `masks(key, created_at)`,
  migrations idempotentes à chaque ouverture, API idempotente
  (`AddMask`/`RemoveMask`/`MaskedKeys`) ;
- `internal/report/` — vues `Finding`/`Data`, `RenderHTML`
  (template embarqué `go:embed`, échappement XSS natif), `JSON()` ;
- `internal/ui/server.go` — interface `Store` définie côté consommateur,
  routes `GET /`, `POST /mask`, `POST /unmask` (redirect 303) ;
- `cmd/phperf` — flags `--profile --rules [--scoring] [--db] [--addr]`,
  pipeline en mémoire, conversion vers les vues report ; service compose
  `web` mis à jour (fixture de démo, `-buildvcs=false`) ;
- cibles make `tests-storage`, `tests-report`, `tests-ui`.

### Jalon 6 — Expressivité du moteur + catalogue de règles — FAIT

Décisions validées avec le pilote (23/08/2026) :

- **La bibliothèque de règles est un livrable de connaissance en soi**,
  pas un sous-produit du profilage réel : enrichie depuis la connaissance
  des anti-patterns PHP classiques, pas « en découvrant le bruit » sur une
  app. Le profilage réel servira ensuite à valider/ajuster, pas à démarrer.
- **Chargement multi-fichiers (packs) refusé pour l'instant** : le
  catalogue tient dans un fichier ; les packs par framework resteront un
  jalon à part.
- **Pilotage par user story de détection** : chaque nouveau critère de
  match est justifié par des règles concrètes qu'il débloque.

Moteur (`internal/rules`, `internal/analyzer`) :

- nouveaux critères `Match` : `inclusive_wt_ms_threshold`,
  `exclusive_wt_ms_threshold` (temps propre), `time_share_percent_threshold`
  (part de trace 0–100), `caller_count_threshold` (fan-out),
  `peak_memory_per_call_threshold_mb` (ΣPMU/ΣCT) et `exclude_pattern`
  (anti faux positifs — ne compte pas comme critère déclencheur seul) ;
- deux périmètres documentés : **par site d'appel** (pattern, call_count)
  vs **par callee agrégé** (mémoire, temps, fan-out) — unités seuils en
  ms/%/Mo, conversion interne µs/octets faite par le moteur ;
- `analyzer.Edge` porte désormais PMU (pic mémoire propagé du brut) ;
- refactor moteur : agrégats par callee calculés paresseusement une fois
  par évaluation (`statsCache`), critères évalués dans `Match.satisfied`.

Catalogue (`proto/rules.example.yaml`) : 20 règles organisées par familles —

- amplification d'I/O : n-plus-one-query, slow-single-query (complément :
  peu d'appels mais lourd), network-call-in-loop, filesystem-io-in-loop,
  cache-chatter-in-loop, mail-send-in-loop ;
- hotspots temporels : cpu-bound-function (temps propre, I/O exclues),
  dominant-subtree (≥ 25 % de la trace) ;
- sérialisation/traitement : serialization-hotspot, regex-heavy-work,
  template-render-heavy ;
- mémoire : memory-heavy-allocation, memory-spike-transient (PMU) ;
- code smells au profil : debug-leftovers, blocking-sleep,
  array-merge-in-loop, linear-scan-in-loop, slow-hash-repeated,
  duplicated-calculation (heuristique faible assumée) ;
- ORM : doctrine-hydration-storm.

Conformité : chaque famille a sa fixture dédiée (`scripts/fixtures/
{hotspot,io-heavy,smells,spike}.json`) et `TestCatalogAgainstFixtures`
asserte les listes exactes de findings attendus (trigger ET non-trigger)
sur les 7 fixtures.

---

## Jalons futurs (propositions détaillées — bilan avant mise en ligne, 23/08/2026)

### Jalon 7 — Collecte PHP réelle : primitives PHP + documentation — FAIT

Décisions (24/08/2026) :

- **Séquence en deux jalons** : primitives PHP autonomes d'abord
  (fonctionnelles immédiatement), **package Composer ensuite** (jalon 9)
  qui les emballera avec des adaptateurs framework — l'expérience
  zéro-code est repoussée à ce second jalon, pas abandonnée.
- **Pas de sous-commande Go `phperf collect`** pour l'instant : le wrapper
  PHP affiche ses propres erreurs ; l'orchestration Go attendra qu'il y ait
  une commande unique à orchestrer (après le package Composer).
- **CLI + HTTP dès ce jalon** ; la paire prepend/append reste documentée
  comme repli pour legacy sans Composer même après le jalon 9.

Livrables :

- `scripts/php/phperf-profile.php` — wrapper CLI générique : vérif
  ext-xhprof (message pecl), flags CPU+MEMORY par défaut et **builtins
  conservés** (les règles ciblent `array_merge`, `hash`, `usleep`…),
  passage d'args après `--`, profil partiel écrit même si le scénario
  lève (exit 1), normalisation du dump (entiers, champs garantis,
  racine `main()` synthétisée si absente) ;
- `scripts/php/phperf-prepend.php` / `phperf-append.php` — profilage HTTP
  sans toucher au code : déclenché uniquement si `PHPERF_PROFILE=1`
  (requêtes ordinaires gratuites), dump horodaté dans `PHPERF_OUTPUT_DIR`,
  l'amorce inclut elle-même l'append (auto_prepend_file seul suffit) ;
- `scripts/fixtures/php-demo/` + image `Dockerfile.demo` + cible
  `make demo-collect` — essai bout-en-bout sans framework ;
- compose `web` paramétrable (`PHPERF_PROFILE=bin/phperf-demo.json make up`) ;
- `docs/utilisation.md` étape 1 réécrite (installation ext-xhprof, scénario
  CLI, mode HTTP, exemple GitHub Actions réel).

Vérification sur profil réel (image php:8.3-cli + xhprof) : le scénario de
démo produit un profil de 10 entrées qui déclenche **7 règles distinctes**
(12 findings) — dont n-plus-one-query (`Doctrine\DBAL\FakeConnection::query`,
ct=50), duplicated-calculation ×5, dominant-subtree ×2, blocking-sleep,
array-merge-in-loop, slow-single-query, cpu-bound-function ; chaîne
baseline → run → exit 0 verte.

Formalisation en test E2E automatisé :

- `scripts/e2e.sh` + cible `make e2e` (côté hôte, **hors gates**) :
  image php+xhprof → collecte wrapper → assertions du contrat JSON
  (racine main(), champs entiers garantis, N+1 ct=50) → chaîne CI
  (baseline/run exit 0, refus sans baseline) → boot UI + findings rendus.
- Pièges rencontrés : binaires liés à la glibc du conteneur toolchain →
  la chaîne CI tourne aussi en conteneur ; chemins absolus après `cd`
  dans les sous-shells ; `bin/` appartient au conteneur (root) → espace
  de travail E2E à la racine (`.e2e-*`, nettoyé par trap).
- Le dépôt gagne sa propre CI GitHub Actions (`.github/workflows/ci.yml`) :
  job `check` (`make check`) + job `e2e` (`make e2e`) sur chaque PR/push.

### Jalon 8 — Distribution des binaires — FAIT

Objectif : télécharger et exécuter PHPerf sans cloner le dépôt ni avoir
Go d'installé — binaires statiques publics sur GitHub Releases.

Livrables :

- `cmd/phperf/main.go` + `cmd/phperf-ci/commands.go` : flag `--version`
  (cobra pour phperf-ci, `flag` pour phperf) ;
- `make build` : binaires **statiques** (`CGO_ENABLED=0`) avec version
  estampée via `-ldflags "-X main.version=…"` ;
- `scripts/release.sh` : cross-compilation (linux/darwin/windows ×
  amd64/arm64) → archives tar.gz + `SHA256SUMS` dans `dist/` ;
- `.github/workflows/release.yml` : à chaque tag `v*`, `make release` puis
  publication automatique sur GitHub Releases.

L'archive contient les deux binaires (`phperf` + `phperf-ci`) ; extraction
et exécution directe — aucune étape supplémentaire.

---

### Jalon 9 — Package Composer `phperf/profile`

Objectif UX : `composer require phperf/profile` puis une commande unique,
zéro fichier à écrire chez l'utilisateur.

- Autoload `files` : activation très précoce (dès vendor/autoload.php) si
  `PHPERF_PROFILE=1` — supprime la config serveur auto_prepend_file ;
- Vérification proactive d'ext-xhprof avec instructions ;
- Implémentation interne = les primitives du jalon 7 (mêmes garanties JSON).

### Jalon 10 — Dédoublonnage des findings (`supersedes`)

Plusieurs règles peuvent tomber sur **le même callee** en décrivant le
même problème — ex. une requête SQL dans une boucle déclenche à la fois
`n-plus-one-query` (batcher) et `duplicated-calculation` (mémoïser) ;
traiter le premier rend le second sans objet.

- **Option retenue : « supersedes » déclaratif** (pas simple regroupement
  visuel) ;
  - champ `supersedes: [id...]` dans le format de règle : la règle la plus
    spécifique déclare celles qu'elle remplace ;
  - moteur : quand règle R et une règle remplacée S tombent sur le **même
    callee**, le finding de S est retiré (résolution transitive) ;
  - câblage catalogue : `n-plus-one-query` et `slow-single-query`
    remplacent `duplicated-calculation` sur les callees SQL.
- Les paires *complémentaires* restent affichées ensemble
  (ex. `n-plus-one-query` + `dominant-subtree`) : remèdes différents.

### Jalon 11 — Packs de règles multi-fichiers

`--rules proto/rules.d/*.yaml` (ou répertoire) : cœur générique + packs par
framework (Symfony, Laravel, WordPress…) maintenus séparément. Conditionne
le modèle communautaire. Le format v1 reste inchangé ; seul le chargement
s'étend (fusion stricte, ids globalement uniques).

### Post-MVP ( backlog non planifié )

- Backends alternatifs : php-spx (timeline), phpspy (échantillonnage bas
  coût) — l'architecture collector→analyzer est prête.
- Critère d'attente externe : ratio (wt−cpu)/wt pour distinguer calcul et
  I/O-wait dans les hotspots.
- Export masques → baseline (partager le triage local avec l'équipe).
- Suggestions de correctifs assistées (LLM) sur les findings sans reco.
- Rapport HTML autonome exportable (pièce jointe de CI).
- Sous-commande Go `phperf collect` (orchestration wrapper + validation
  JSON) — pertinente une fois le package Composer en place.

## Leçons & conventions acquises en cours de route

- **misspell a corrompu des commentaires français** via `make fix`
  (« respecte »→« respective », « transforme »→« transfer », « normalisé »→
  « normalsé ») → linter retiré ; commentaires réparés. Si une coquille
  bizarre apparaît dans un commentaire, suspecter un reliquat.
- **goconst** : le réglage `goconst.ignore-tests: true` fonctionne en v2 ;
  ne pas passer par `exclusions.rules` pour ça.
- Toujours relire un fichier après un passage de `golangci-lint fmt` avant
  d'éditer (le formatage peut avoir décalé les lignes).

## Reprendre le travail plus tard

- Ce fichier (`docs/jalons.md`) résume l'état complet du projet : le
  coller en contexte d'une nouvelle session — quel que soit l'outil —
  suffit à reprendre. Pointer également vers les `AGENTS.md` (lus
  automatiquement par la plupart des assistants).
