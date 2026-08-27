# PHPerf

Profileur de performance PHP open source & gratuit — libre d'utilisation,
orienté **priorisation explicite des actions correctives**.

PHPerf ne réinvente pas le profilage : il s'appuie sur XHProf pour profiler
une application PHP existante, puis transforme les données brutes en plan
d'action exploitable :

- **Scoring automatique & ajustable** — chaque finding reçoit un score
  `Impact × Effort × Contrôlabilité`, dont les poids sont configurables.
- **Masquage / triage persistant** — un développeur peut masquer un goulot
  jugé non résoluble ; la décision est conservée (SQLite) et ignorée en CI
  tant que le finding n'est pas démasqué.
- **Suggestions de correctifs** — chaque finding est accompagné d'une
  recommandation concrète (cache, refacto, queue…) via un moteur de règles YAML.
- **Baseline CI (à la PHPStan)** — la première passe enregistre les findings
  connus dans une baseline ; en CI, `phperf-ci` n'échoue que si des findings
  nouveaux apparaissent par rapport à elle.

## État du projet

**Utilisable sur profils JSON.** Le pipeline complet est implémenté et
testé (jalons 0–6) : normalisation XHProf, moteur de règles (catalogue de
20 règles), scoring ajustable, baseline CI, UI web avec masquage persistant.
La collecte automatique depuis une application PHP réelle est le prochain
jalon (7) — voir [`docs/jalons.md`](docs/jalons.md).

## Démarrage rapide

**Option 1 — Binaires (recommandé)** : télécharger depuis
[GitHub Releases](https://github.com/phperf/phperf/releases), extraire et
lancer. Aucun prérequis.

**Option 2 — Depuis la source** : Go 1.21+ requis, ou Docker uniquement
(la toolchain Go, golangci-lint et goimports sont fournis par l'image du
`Dockerfile` — version maîtrisée, rien à installer sur l'hôte).

```bash
make check    # lint + vet + tests — doit passer avant commit
make build    # compile bin/phperf et bin/phperf-ci (statiques)
make e2e      # test E2E collecte PHP réelle (côté hôte, ~2 min à froid)
make release  # cross-compile archives pour GitHub Releases
make up       # interface web de démonstration sur http://localhost:8080
```

**Option 3 — Collecte PHP** (si vous voulez profiler votre app sans toucher
à votre code) :
```bash
composer require --dev ph-perf/profile
# Puis PHPERF_PROFILE=1 devant la commande/requete à profiler.
```
Voir [`docs/utilisation.md`](docs/utilisation.md) pour les détails.

## Utilisation visée

### Baseline en CI

À la manière de PHPStan : la première passe enregistre les problèmes connus
dans une baseline, ensuite la CI ne juge que la régression.

```bash
phperf-ci baseline --profile=profil.json --rules=proto/rules.example.yaml   # régénère la baseline (.phperf-baseline.json)
phperf-ci run --profile=profil.json --rules=proto/rules.example.yaml        # exit 1 sur les nouveaux findings
```

`--profile` est un profil XHProf sérialisé en JSON ; `--rules` un fichier
de règles YAML (`proto/rules.example.yaml` comme modèle) ; `--scoring`
permet d'ajuster les pondérations de priorité (optionnel).

La baseline se versionne avec le projet ; les findings qu'elle contient sont
considérés comme connus et ignorés jusqu'à correction ou régénération.

| Exit code | Signification |
|---|---|
| `0` | Aucun nouveau finding par rapport à la baseline |
| `1` | Nouveaux findings non couverts par la baseline |
| `2` | Erreur d'exécution (profilage impossible…) |

### Interface web

```bash
phperf --profile=profil.json --rules=proto/rules.example.yaml \
       [--scoring=poids.yaml] [--db=.phperf.db] [--addr=:8080]
# Explorer les findings priorisés, gérer le triage (masquage persistant
# SQLite) dans le navigateur.
```

## Développement : tests & vérifications

```bash
make fix      # corrige formatage + problèmes auto-fixables
make lint     # formatage + analyse statique (gofmt, goimports, golangci-lint)
make tests    # tests unitaires (+ race detector, couverture)
make check    # lint + vet + tests d'un coup
```

Ces vérifications doivent passer à 100 % avant tout commit (détails dans
[`AGENTS.md`](AGENTS.md)). Objectifs qualité sur les packages métier
(`analyzer`, `rules`, `scorer`) : couverture **100 %** et score de mutation
**≥ 90 %**.

## Règles YAML

Le moteur de règles est déclaratif et extensible — **20 règles livrées**
(N+1, hotspots CPU, I/O répétées, pics mémoire, résidus de debug…) :

- Catalogue complet commenté : [`proto/rules.example.yaml`](proto/rules.example.yaml)
- Schéma de validation : [`proto/rules.schema.json`](proto/rules.schema.json)
- Critères disponibles : temps inclusif/exclusif, part de trace, mémoire
  moyenne et pic, comptages d'appels/sites, patterns avec exclusions.

## Architecture en un coup d'œil

```
[App PHP cible] → collector → analyzer → rules → scorer → storage (SQLite)
                                                            ↓
                                        UI web (phperf) · rapports · phperf-ci (baseline)
```

Détail du pipeline, des responsabilités par composant et du modèle de scoring :
[`docs/architecture.md`](docs/architecture.md).

## Structure du dépôt

```
cmd/                 points d'entrée (thin wrappers) : phperf (web), phperf-ci (CI)
internal/            code métier : collector, analyzer, rules, scorer, storage, ui, report
proto/               format déclaratif des règles YAML (exemple + schéma JSON)
scripts/             fixtures XHProf, scripts utilitaires (build, setup, seed)
docs/                documentation utilisateur
Dockerfile           image dev : golang:1.27 + golangci-lint épinglé
docker-compose.yml   services app (outils) et web (UI, port 8080)
Makefile             cibles make : fix, lint, tests, check, build, up…
```

## À propos des fichiers AGENTS.md

Chaque dossier contient un `AGENTS.md` destiné aux **assistants IA**
(conventions, interdits, références croisées). Ils ne remplacent pas cette
documentation : ils encadrent la contribution générée par IA. Les humains
trouveront l'essentiel ici et dans [`docs/`](docs/).
