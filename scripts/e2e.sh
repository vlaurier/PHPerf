#!/bin/sh
# PHPerf — test E2E de la chaîne de collecte (côté hôte, opt-in : make e2e).
#
# Valide le contrat complet sur un profil produit par un vrai runtime PHP :
#   image php+xhprof → wrapper CLI → assertions JSON → phperf-ci
#   (baseline/run, refus sans baseline) → UI web (findings rendus).
#
# Prérequis : Docker + réseau + curl. Hors quality gates (durée ~2 min à
# froid, secondes ensuite grâce au cache des couches d'image).
set -eu

ROOT=$(pwd)
WORK=$ROOT/.e2e-$$
PORT=$(( ($(date +%s) + $$) % 20000 + 30000 ))
IMAGE=phperf-demo-php

cleanup() {
    cd "$ROOT"
    docker compose down >/dev/null 2>&1 || true
    rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

step() { printf '\n== %s ==\n' "$1"; }
fail() {
    printf '\nE2E ÉCHEC : %s\n' "$1" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl requis"
command -v docker >/dev/null 2>&1 || fail "docker requis"
mkdir -p "$WORK"

step "1/5 Image PHP + ext-xhprof"
docker build -q -t "$IMAGE" -f scripts/php/Dockerfile.demo scripts/php >/dev/null \
    || fail "build image démo"

step "2/5 Collecte réelle via le wrapper CLI"
docker run --rm -v "$ROOT":/work -w /work -v "$WORK":/out "$IMAGE" \
    php scripts/php/phperf-profile.php --output=/out/profile.json \
    scripts/fixtures/php-demo/scenario.php >"$WORK/collect.log" 2>&1 \
    || { cat "$WORK/collect.log"; fail "exécution du wrapper"; }
grep -q "profil écrit" "$WORK/collect.log" || fail "wrapper : confirmation absente"

step "3/5 Contrat JSON du profil"
docker run --rm -v "$WORK":/out "$IMAGE" php -r '
$p = json_decode(file_get_contents("/out/profile.json"), true);
if (!is_array($p)) { fwrite(STDERR, "JSON invalide\n"); exit(1); }
if (!isset($p["main()"])) { fwrite(STDERR, "racine main() absente\n"); exit(1); }
foreach ($p as $key => $row) {
    foreach (["ct", "wt", "cpu", "mu", "pmu"] as $f) {
        if (!isset($row[$f]) || !is_int($row[$f])) {
            fwrite(STDERR, "champ $f manquant ou non entier : $key\n"); exit(1);
        }
    }
}
$k = "main()==>Doctrine\DBAL\FakeConnection::query";
if (($p[$k]["ct"] ?? 0) !== 50) { fwrite(STDERR, "N+1 attendu (ct=50) sur $k\n"); exit(1); }
if (count($p) < 8) { fwrite(STDERR, "profil anormalement pauvre\n"); exit(1); }
printf("%d entrées — contrat respecté\n", count($p));
' || fail "assertions sur le profil"

step "4/5 Chaîne CI : baseline puis run"
# Les binaires sont liés à la glibc du conteneur toolchain : la chaîne CI
# s'exécute donc dedans également ($WORK est monté sous /workspace).
RULES=/workspace/proto/rules.example.yaml
make build >/dev/null
docker compose run --rm -e E2E_WORK=".e2e-$$" app \
    bash -ec 'cd "$E2E_WORK" && /workspace/bin/phperf-ci baseline --profile=profile.json --rules='"$RULES" >/dev/null \
    || fail "phperf-ci baseline"
docker compose run --rm -e E2E_WORK=".e2e-$$" app \
    bash -ec 'cd "$E2E_WORK" && /workspace/bin/phperf-ci run --profile=profile.json --rules='"$RULES" >/dev/null \
    || fail "phperf-ci run avec baseline fraîche (exit 0 attendu)"
rm "$WORK/.phperf-baseline.json"
if docker compose run --rm -e E2E_WORK=".e2e-$$" app \
    bash -ec 'cd "$E2E_WORK" && /workspace/bin/phperf-ci run --profile=profile.json --rules='"$RULES" >/dev/null 2>&1; then
    fail "phperf-ci run sans baseline aurait dû refuser"
fi
printf 'baseline -> run : exit 0 ; sans baseline : refusé\n'

step "5/5 Interface web sur le profil réel"
PHPERF_PROFILE=".e2e-$$/profile.json" PHPERF_PORT=$PORT make up >/dev/null
tries=0
while :; do
    body=$(curl -fsS "http://localhost:$PORT/" 2>/dev/null || true)
    case $body in
        *n-plus-one-query*) break ;;
    esac
    tries=$((tries + 1))
    [ "$tries" -gt 30 ] && fail "UI sans finding n-plus-one-query (port $PORT)"
    sleep 2
done
case $body in
    *array-merge-in-loop*) ;;
    *) fail "UI sans finding array-merge-in-loop" ;;
esac
printf "UI ok sur http://localhost:%s (findings attendus présents)\n" "$PORT"

step "6/6 Package Composer : autoload-activated profiling"
docker run --rm -v "$ROOT":/src -w /src "$IMAGE" sh -ec '
set -e
APPDIR=/tmp/phperf-app
rm -rf "$APPDIR" && mkdir -p "$APPDIR/app"
cat > "$APPDIR/composer.json" <<EOF
{
    "minimum-stability": "dev",
    "prefer-stable": true,
    "repositories": [
        { "type": "path", "url": "/src/php" }
    ],
    "require": {
        "phperf/profile": "*"
    }
}
EOF
cat > "$APPDIR/app/index.php" <<'"'"'PHPEOF'"'"'
<?php
require __DIR__."../../vendor/autoload.php";
for ($i=0; $i<20; $i++) { array_merge(range(0,$i), range(0,$i)); }
PHPEOF
cd "$APPDIR"
composer install --no-interaction 2>&1 || { cat composer.json; echo "---" ; exit 1; }
PHPERF_PROFILE=1 PHPERF_OUTPUT_DIR="$APPDIR" php app/index.php 2>&1 || { echo "php failed" >&2; exit 1; }
PROFILE=$(ls "$APPDIR"/phperf-*.json 2>/dev/null | head -1)
[ -n "$PROFILE" ] && [ -s "$PROFILE" ] || { ls "$APPDIR"; echo "JSON absent" >&2; exit 1; }
php -r "\$p=json_decode(file_get_contents(\"$PROFILE\"),true); if(!isset(\$p[\"main()\"])) exit(1);" \
    || { echo "main() absente" >&2; exit 1; }
printf "Package Composer validé : profil %s octets\n" "$(wc -c < "$PROFILE")"
' 2>&1 || fail "package Composer phperf/profile"

printf '\nE2E PASS — chaîne complète validée.\n'
