# PHPerf — environnement de développement.
# Go 1.27 : dernière version soutenue (19/08/2026). Image officielle, version
# maîtrisée par ce fichier — rien à installer sur l'hôte à part Docker.

FROM golang:1.27-bookworm

# golangci-lint épinglé (v2.13.1 = première version compatible Go 1.27).
ARG GOLANGCI_LINT_VERSION=v2.13.1
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/main/install.sh \
      | sh -s -- -b "$(go env GOPATH)/bin" "${GOLANGCI_LINT_VERSION}"

# goimports sur @latest (outil de formatage stable) — épinglable via
# --build-arg GOIMPORTS_VERSION=vX.Y.Z si besoin de reproductibilité stricte.
ARG GOIMPORTS_VERSION=latest
RUN go install "golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION}"

WORKDIR /workspace
