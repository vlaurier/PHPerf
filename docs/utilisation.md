# Guide d'utilisation — PHPerf

> Mode opératoire pour utiliser PHPerf sur une application PHP.
> Les rares briques encore à venir sont signalées « À venir ».

## Prérequis

| Composant | Rôle | Notes |
|---|---|---|
| Docker | Exécuter PHPerf et sa toolchain | Seul prérequis côté hôte |
| PHP 8.x + extension `xhprof` | Produire le profil | Côté application profilée — installation ci-dessous |

PHPerf n'est **pas** (encore) une dépendance Composer de votre application :
c'est un outil externe qui lit un profil XHProf exporté en JSON. Votre code
métier reste intouché — seuls deux fichiers de colle légers interviennent
(scénario CLI ou config serveur HTTP).

## Étape 1 — Obtenir un profil

Un « profil » est le dump XHProf de votre application, sérialisé en JSON
(clés `parent==>enfant`, valeurs `ct/wt/cpu/mu/pmu`). PHPerf livre les
scripts de collecte dans [`scripts/php/`](../scripts/php/) — copiez-les
quelque part accessible par l'application (ou clonez le dépôt).

### (a) Installer ext-xhprof (une seule fois, côté infra)

```bash
pecl install xhprof
echo 'extension=xhprof.so' > "$(php -ini | awk '/^Scan this dir/{print $NF}')/xhprof.ini"
php -m | grep xhprof   # doit afficher xhprof
```

Sous Docker : `RUN pecl install xhprof && docker-php-ext-enable xhprof`.
L'extension ne coûte rien tant qu'elle n'est pas activée ; activez-la
uniquement sur les environnements où vous profilez (dev, CI…).

### (b) Tâche CLI : le fichier scénario

Créez `phperf-scenario.php` **à la racine de votre projet** — il boote
l'application et lance la charge à profiler :

```php
<?php
require __DIR__.'/vendor/autoload.php';
// Boot maison (exemple Symfony) :
$kernel = new App\Kernel('dev', true); $kernel->boot();
// La charge à profiler :
system('php bin/console app:rapport-quotidien');
```

Puis :

```bash
php /chemin/phperf-profile.php --output=profil.json phperf-scenario.php
```

Options utiles : `--no-cpu`, `--no-memory`, `--no-builtins` (déconseillé :
plusieurs règles ciblent des fonctions natives). Un scénario qui plante
produit quand même un profil partiel (exit code 1). Essayez sans rien
installer avec la démo fournie :

```bash
make demo-collect   # → bin/phperf-demo.json via une image php+xhprof éphémère
```

### (c) Pages HTTP : zéro modification de code

Déclarez l'amorce dans la config PHP du serveur (php.ini, pool FPM,
`.htaccess` selon votre montage) :

```ini
auto_prepend_file = /chemin/phperf-prepend.php
```

C'est tout : l'amorce inclut elle-même la fin de chaîne. Le profilage ne
s'active que si la variable d'environnement `PHPERF_PROFILE=1` est posée
(sur une requête, un pod, un conteneur…) — les requêtes ordinaires ne paient
rien. Chaque requête profile écrit un JSON horodaté dans
`PHPERF_OUTPUT_DIR` (défaut : tmp système) et le signale dans les logs :

```bash
PHPERF_PROFILE=1 curl -s http://localhost:8080/rapports > /dev/null
# → log "[phperf] profil écrit : /tmp/phperf-20260824-141233-a1b2c3.json"
```

### (d) Consommer le profil

Tout ce qui suit accepte n'importe quel profil JSON obtenu ci-dessus :
UI web (étape 3), baseline CI (étape 4). Les fixtures de référence du
format sont dans [`scripts/fixtures/`](../scripts/fixtures/).

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
      - uses: shivammathur/setup-php@v2
        with: { php-version: '8.3', extensions: xhprof }
      - uses: actions/setup-go@v5
        with: { go-version: stable }
      - run: go install github.com/phperf/phperf/cmd/phperf-ci@latest
      # Wrapper de collecte (voir étape 1b) + scénario versionné chez vous.
      - run: curl -fsSL https://raw.githubusercontent.com/phperf/phperf/main/scripts/php/phperf-profile.php -o phperf-profile.php
      - run: php phperf-profile.php --output=profil.json phperf-scenario.php
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

1. **Profiler** l'app — scénario CLI ou variable d'env HTTP (étape 1).
2. **Explorer** les findings priorisés dans l'UI, masquer le bruit local.
3. Corriger les priorités hautes ; **régénérer la baseline** quand c'est
   assumé.
4. Laisser la **CI** garantir qu'aucun nouveau goulot n'entre sans revue.

> À venir (jalon 8) : un package Composer `phperf/profile` absorbant la
> configuration serveur et le fichier scénario pour les frameworks supportés
> (Symfony, Laravel…) — `composer require` puis une commande unique.
