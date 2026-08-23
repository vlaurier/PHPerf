# AGENTS.md — PHPerf

> Instructions globales pour l'IA — valables à la racine du projet.
> Chaque sous-dossier contient son **propre** `AGENTS.md` avec des consignes
> **spécifiques** à ce dossier. Ne pas y mettre de doublons.

## 1. Vision produit

PHPerf est un **profileur de performance PHP open source & gratuit**,
libre d'utilisation pour tous. Son originalité :

  - **Scoring automatique & ajustable** (Impact × Effort × Contrôlabilité)
  - **Système de masquage / triage** persistant (SQLite)
  - **Suggestions de correctifs** concrètes (moteur de règles YAML)
  - **Intégration CI via baseline** (à la PHPStan) — `phperf-ci` échoue
    uniquement sur les findings nouveaux, pas sur l'existant

Le produit ne profile ni n’analyse du code : il **profite** un code existant
via XHProf (ou futurs backing: php-spx, phpspy) et produit une
**priorisation exploitable**.

## 2. Stack technique

| Composant | Tech | Justification |
|---|---|---|
| Langage | **Go 1.21+** (toolchain 1.27 en conteneur) | Binaire unique pour CI, syntaxe claire pour IA |
| Environnement | **Docker** (`golang:1.27`) | Version maîtrisée par le `Dockerfile` ; rien à installer sur l'hôte |
| Orchestration dev | **Make** | Entrées simples : `make fix`, `lint`, `tests`, `check`, `up`… |
| Profiler backing (initial) | **XHProf** | Widespread, format stable, licence Apache 2.0 |
| Moteur de règles | **YAML** | Configurable, extensible, communautaire |
| Persistance | **SQLite** | Fichier unique, pas de daemon, persistance du masquage |
| Linter / analyse | **golangci-lint** | Un seul outil = 30+ checks (gofmt, staticcheck, etc.) |
| Tests | `go test` + **testify** | Standard Go, assertions riches |
| Formatage | `gofmt` / `goimports` | Natif, non configurable |
| CLI | `spf13/cobra` | Standard des CLI Go |
| YAML | `gopkg.in/yaml.v3` | Parser YAML natif, support tags |

## 3. Structure du projet

```
PHPerf/
├── AGENTS.md                    ← [CE FICHIER] instructions globales
├── .golangci.yml                ← config golangci-lint (format v2)
├── Makefile                     ← cibles dev exécutées en conteneur
├── Dockerfile                   ← image dev : golang:1.27 + golangci-lint épinglé
├── docker-compose.yml           ← services : app (outils), web (UI, port 8080)
├── go.mod / go.sum
├── cmd/                         ← points d'entrée (binaires)
│   ├── phperf/                  ← web UI (serveur HTTP)
│   └── phperf-ci/               ← CLI: CI avec baseline
├── internal/                    ← code non public (conventions Go)
│   ├── collector/               ← collecte XHProf + orchestration
│   ├── analyzer/                ← normalisation du call graph
│   ├── rules/                   ← moteur de règles YAML
│   ├── scorer/                  ← calcul du score de priorité
│   ├── baseline/                ← fichier baseline CI + diff nouveaux findings
│   ├── storage/                 ← SQLite (profils, findings, masquage) — jalon 5
│   ├── ui/                      ← handlers HTTP + templates
│   └── report/                  ← génération de rapports (HTML, JSON)
├── proto/                       ← schéma/example des règles
│   └── rules.example.yaml
├── scripts/                     ← scripts utiles (fixtures, build)
└── docs/                        ← docs utilisateur (hors AGENTS.md)
```

> Convention Go : tout le code métier est dans `internal/`. Les binaires
> (`cmd/phperf` et `cmd/phperf-ci`) sont **thin wrappers** — aucune logique
> métier ne s'y trouve.

### Dépendances autorisées entre packages (esprit deptrac)

Pas de domaine riche ici, mais une **séparation stricte en couches** : le
graphe de dépendances est un DAG orienté du bas vers le haut, chaque couche
ne connaissant que la couche immédiatement inférieure.

```
cmd/*  →  tout internal/ (wiring uniquement, zéro logique)

collector ──✗──  aucune dépendance interne
analyzer  →  collector
rules     →  analyzer
scorer    →  rules
baseline  →  rules
storage   ──✗──  définit ses propres modèles (pas d'import métier)
ui        →  storage, report
report    →  storage
```

- Interdits : dépendance inverse, import en diagonale (ex : `scorer` →
  `analyzer`), cycle entre packages.
- **Appliqué mécaniquement** par le linter `depguard` (`.golangci.yml`) :
  créer une nouvelle dépendance impose de modifier la matrice ici ET là-bas
  — c'est voulu, ça force la discussion.

## 4. Commandes (quality gates)

Tout s'exécute **en conteneur** via le Makefile — seul Docker est requis sur
l'hôte :

```bash
make fix      # corrige formatage + problèmes auto-fixables
make lint     # formatage + analyse statique (gofmt, goimports, golangci-lint)
make vet      # go vet ./...
make tests    # tests unitaires (+ race detector, couverture)
make check    # lint + vet + tests — doit passer à 100 % avant commit
make build    # compile bin/phperf et bin/phperf-ci
make up       # interface web sur http://localhost:8080
make shell    # shell dans le conteneur dev
```

Équivalents bruts (à lancer **dans le conteneur**, ex :
`docker compose run --rm app <cmd>`) :

```bash
# Formatage (obligatoire avant commit)
gofmt -l . && goimports -l .

# Lint + static analysis (bloque la CI si échoue)
golangci-lint run --timeout=5m

# Vérifications supplémentaires
go vet ./...

# Tests unitaires (+ race detector)
go test -race -cover ./...

# Build
go build -o bin/phperf ./cmd/phperf
go build -o bin/phperf-ci ./cmd/phperf-ci

# Générer les mocks (si nécessaire)
go generate ./...
```

**golangci-lint** = gofmt + goimports + govet + staticcheck + errcheck +
ineffassign + gosimple + unparam + misspell + revive + ...
(voir `.golangci.yml` pour la config).

## 5. Conventions de code

- **Formatage** : `gofmt`/`goimports` à 100% — jamais de code non formaté.
- **Nommage** : `camelCase` pour les vars/methods, `PascalCase` pour exports.
- **Gestion d'erreurs** : toujours `if err != nil { return ... }` — pas d'ignorés.
- **SOLID** (adapté à Go) :
  - *SRP* — un package = une responsabilité ; une fonction fait une chose.
  - *OCP / LSP* — étendre via de nouvelles implémentations d'interface
    (`Collector`, `Normalizer`, `Generator`…), jamais en modifiant le cœur.
  - *ISP* — interfaces petites (1–3 méthodes), définies **côté consommateur**
    (« accept interfaces, return structs »).
  - *DIP* — dépendre d'abstractions ; injection par constructeurs `New...()` ;
    pas de variables globales ni de singletons.
- **Complexité cyclomatique** : ≤ 15 par fonction, objectif ~10 — appliqué
  par `cyclop` (`.golangci.yml`). Au-delà : extraire une fonction/classe.
- **Tests** : **table-driven** (structure `{name, input, want}`) avec
  `testify`. Objectifs sur les packages métier (`analyzer`, `rules`,
  `scorer`) :
  - couverture **100 %** (`make tests`) ;
  - score de mutation **≥ 90 %** avant merge (outil type gremlins).
- **Contexte** : passer `context.Context` en 1re arg des fonctions I/O.
- **Doc** : commenter chaque export avec `// Nom — description`.
- **Dépendances** : justifier chaque import externe. Préférer stdlib.

## 6. Directives pour l'IA

1. **Avant d'écrire**, lis les `AGENTS.md` du dossier courant ET ceux des
   parents. Agis uniquement sur ce qui concerne le dossier travaillé.
2. **N'invents pas de structure** — propose une structure dans le PR / message
   avant de coder.
3. **Tests d'abord pour la logique métier** (analyzer, scorer, rules).
4. **Pas de code métier dans `cmd/`** — seulement le wiring (DI, flags).
5. **Règles YAML exemples** dans `proto/`, jamais de logique dans le format.
6. **Quand tu bloques** : écris `// TODO(phperf): <question>` et note la
   question dans ton message final. Ne devine pas les assumptions business.
7. **Quality gates non négociables** : `gofmt`, `golangci-lint`, `go test`
   doivent passer à 100% à chaque changement.

## 7. Règles (non-exhaustives) — à enrichir

- **Messages de commit en anglais** (une ligne, impératif, sans préfixe de
  jalon superflu si le message se suffit).

- XHProf comme backing par défaut ; architecture prévue pour plugins
  (php-spx, phpspy).
- SQLite fichier unique — schema dans `internal/storage/`.
- Priorité au **developer experience** : CLI ergonomique, messages clairs.
- Tout le développement passe par les cibles `make` (conteneur) — ne pas
  supposer que Go ou golangci-lint sont installés sur l'hôte.
- Pas de feature non justifiée par un user story.
- **État d'avancement & plan des jalons** : `docs/jalons.md` — source de
  vérité à mettre à jour à chaque jalon terminé ; à lire en début de session.
