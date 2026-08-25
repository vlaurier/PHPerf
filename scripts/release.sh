#!/bin/sh
# PHPerf — construction des archives de release (multi-plateforme).
#
# Produit dans dist/ une archive tar.gz par plateforme contenant les deux
# binaires, plus un fichier SHA256SUMS. Binaires statiques : modernc.org/sqlite
# est pure Go, CGO n'est pas nécessaire.
#
# Usage : sh scripts/release.sh [version] [répertoire]
#   version     étiquette estampée dans les binaires (défaut : dev)
#   répertoire  sortie (défaut : dist)
set -eu

VERSION=${1:-dev}
DIR=${2:-dist}
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

TAG=$(printf '%s' "$VERSION" | tr '/' '_')
rm -rf "$DIR"
mkdir -p "$DIR"

for p in $PLATFORMS; do
    os=${p%/*}
    arch=${p#*/}
    ext=""
    [ "$os" = "windows" ] && ext=.exe
    out="$DIR/phperf-$TAG-$os-$arch"
    mkdir -p "$out"
    for bin in phperf phperf-ci; do
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
            go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$VERSION" \
            -o "$out/$bin$ext" "./cmd/$bin"
    done
done

cd "$DIR"
for d in phperf-*/; do
    tar czf "${d%/}.tar.gz" "$d"
    rm -rf "$d"
done
sha256sum phperf-*.tar.gz > SHA256SUMS
printf 'Release %s prête dans %s :\n' "$VERSION" "$PWD"
ls -1 .
