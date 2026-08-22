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

**En construction.** La structure, les conventions et les formats sont posés ;
l'implémentation des packages `internal/` n'a pas encore commencé. Les
commandes ci-dessous décrivent l'usage visé du produit.

## Démarrage rapide

Prérequis : **Docker uniquement.** La toolchain Go (1.27), golangci-lint et
goimports sont fournis par l'image du `Dockerfile` — version maîtrisée, rien
à installer sur l'hôte.

```bash
make check    # lint + vet + tests — doit passer avant commit
make build    # compile bin/phperf et bin/phperf-ci dans le conteneur
make up       # interface web sur http://localhost:8080
```

Toutes les cibles (`make help`) lancent les outils **dans le conteneur** :
`fix`, `lint`, `vet`, `tests`, `check`, `build`, `shell`, `up`, `down`…

## Utilisation visée

### Baseline en CI

À la manière de PHPStan : la première passe enregistre les problèmes connus
dans une baseline, ensuite la CI ne juge que la régression.

```bash
phperf-ci baseline --script=test_request.php   # génère / met à jour la baseline (.phperf-baseline.json)
phperf-ci run --script=test_request.php --baseline=.phperf-baseline.json
```

La baseline se versionne avec le projet ; les findings qu'elle contient sont
considérés comme connus et ignorés jusqu'à correction ou régénération.

| Exit code | Signification |
|---|---|
| `0` | Aucun nouveau finding par rapport à la baseline |
| `1` | Nouveaux findings non couverts par la baseline |
| `2` | Erreur d'exécution (profilage impossible…) |

### Interface web

```bash
bin/phperf
# Explorer les profils, consulter les findings priorisés,
# gérer le triage (masquage) dans le navigateur.
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

Le moteur de règles est déclaratif et extensible :

- Exemple complet commenté : [`proto/rules.example.yaml`](proto/rules.example.yaml)
- Schéma de validation : [`proto/rules.schema.json`](proto/rules.schema.json)

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
