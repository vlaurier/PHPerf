# Guide d'utilisation — PHPerf

> Mode opératoire cible pour utiliser PHPerf sur une application PHP.
> Certaines briques sont encore en jalon (signalées « À venir ») : le guide
> décrit l'expérience finale visée et ce qui fonctionne dès aujourd'hui.

## Prérequis

| Composant | Rôle | Notes |
|---|---|---|
| Docker | Exécuter PHPerf et sa toolchain | Seul prérequis côté hôte |
| PHP + extension `xhprof` | Produire le profil | Côté application profilée (À venir, jalon 7, pour la collecte automatique) |

PHPerf n'est **pas** une dépendance Composer de votre application : c'est un
outil externe qui lit un profil XHProf exporté en JSON. Votre code reste
intouché.

## Étape 1 — Obtenir un profil

Un « profil » est le dump XHProf de votre application, sérialisé en JSON
(clés `parent==>enfant`, valeurs `ct/wt/cpu/mu/pmu`).

**À venir (jalon 7)** — collecte intégrée :

```bash
# Modalité visée : script autonome, zéro impact sur l'app
phperf collect -- php bin/console app:rapport-quotidien > profil.json
```

Deux modalités à l'étude (script autonome recommandé d'abord,
instrumentation applicative ensuite) — voir `docs/jalons.md`, jalon 7.

**Dès maintenant** — si vous disposez déjà d'un dump XHProf (serialize ou
array), convertissez-le en JSON plat ; les fixtures de référence du format
sont dans [`scripts/fixtures/`](../scripts/fixtures/). L'UI de démonstration
du dépôt consomme la fixture `nplus1.json` :

```bash
make up   # http://localhost:8080 sur la fixture de démo
```

## Étape 2 — Comprendre le rapport

Chaque ligne = un **finding** : une règle YAML qui a détecté un anti-pattern
sur une fonction précise.

- **Priorité (0–100)** = `100 × part_de_temps × poids_effort × poids_maîtrise`.
  La donnée mesurée (temps) domine ; effort et maîtrise ne font que
  départager — ils sont déclarés par l'auteur de la règle, jamais calculés.
- **Gravité** : opinion qualitative de la règle, affichée mais hors formule.
- **Recommandation** : piste concrète de correction propre au pattern.

Les règles livrées (20) couvrent : requêtes N+1, requêtes lourdes, I/O
réseau/fichiers/cache/mail répétées, hotspots CPU, sous-arbres dominants,
sérialisation/regex/templates coûteux, pics mémoire, résidus de debug,
`sleep` bloquant, fusions/scan de tableaux en boucle, hachages répétés…
Catalogue complet commenté :
[`proto/rules.example.yaml`](../proto/rules.example.yaml).

## Étape 3 — Trier avec l'interface web

```bash
bin/phperf --profile=profil.json --rules=proto/rules.example.yaml \
           [--scoring=mes-poids.yaml] [--db=.phperf.db] [--addr=:8080]
```

- Chaque finding propose un bouton **Masquer** (faux positif, won't fix…) ;
  la décision est persistée dans la base SQLite `--db` et survit aux
  relances. **Démasquer** est toujours possible via la vue dédiée
  (`?show_masked=1`).
- Le masquage est un état **local et personnel** : il ne se versionne pas.
- Port personnalisé : flag `--addr=:9090`, ou variable `PHPERF_PORT=9090`
  avec `docker compose up web`.

## Étape 4 — Brancher la CI (baseline façon PHPStan)

La CI ne juge que la **régression** : les problèmes existants vivent dans
une baseline versionnée, les nouveaux findings font échouer le build.

```bash
# Première fois / après avoir corrigé volontairement :
bin/phperf-ci baseline --profile=profil.json --rules=rules.phperf.yaml

# À chaque pull request :
bin/phperf-ci run --profile=profil.json --rules=rules.phperf.yaml
```

| Exit code | Signification |
|---|---|
| `0` | Aucun nouveau finding par rapport à la baseline |
| `1` | Nouveaux findings → échec attendu en CI |
| `2` | Erreur d'exécution (fichier illisible, règles invalides…) |

Exemple GitHub Actions :

```yaml
name: Performance
on: [pull_request]
jobs:
  phperf:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: stable }
      - run: go install github.com/phperf/phperf/cmd/phperf-ci@latest
      # Collecte du profil — À venir (jalon 7). En attendant, fournir
      # un profil JSON par un moyen quelconque :
      - run: ./scripts/generate-profile.sh > profil.json
      - run: phperf-ci run --profile=profil.json --rules=proto/rules.example.yaml
```

Le fichier `.phperf-baseline.json` se **versionne** avec le projet
(contrairement à `.phperf.db`) ; sa régénération est un acte explicite,
à commenter en revue comme une décision.

## Personnalisation

### Règles

Copiez le catalogue dans votre dépôt puis ajustez seuils et patterns :

```bash
cp proto/rules.example.yaml rules.phperf.yaml
$EDITOR rules.phperf.yaml
```

Validation stricte : champ inconnu, id mal formé, regexp invalide ⇒ erreur
au chargement. Schéma JSON de référence :
[`proto/rules.schema.json`](../proto/rules.schema.json). Les critères
disponibles (temps inclusif/exclusif en ms, part de trace en %, mémoire
moyenne/pic en Mo, comptages, patterns) sont documentés en tête du
catalogue.

### Pondérations de score

Optionnel — ajustez la modulation effort/maîtrise sans toucher aux règles :

```bash
cp proto/scoring.example.yaml scoring.yaml && $EDITOR scoring.yaml
bin/phperf-ci run --profile=… --rules=… --scoring=scoring.yaml
```

## Workflow recommandé (résumé)

1. **Profiler** l'app (jalon 7 automatisera cette étape).
2. **Explorer** les findings priorisés dans l'UI, masquer le bruit local.
3. Corriger les priorités hautes ; **régénérer la baseline** quand c'est
   assumé.
4. Laisser la **CI** garantir qu'aucun nouveau goulot n'entre sans revue.
