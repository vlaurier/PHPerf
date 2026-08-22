COMPOSE := docker compose
DC_RUN  := $(COMPOSE) run --rm app

.DEFAULT_GOAL := help
.PHONY: help fix lint vet tidy tests tests-collector tests-analyzer tests-rules check build shell up down logs clean

help: ## Affiche cette aide
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

fix: ## Corrige formatage + problèmes auto-fixables (golangci-lint fmt / run --fix)
	$(DC_RUN) bash -ec 'golangci-lint fmt && golangci-lint run --fix'

lint: ## Vérifie formatage + analyse statique (gofmt, goimports, golangci-lint)
	$(DC_RUN) bash -ec 'golangci-lint fmt --diff && golangci-lint run'

vet: ## go vet ./...
	$(DC_RUN) go vet ./...

tidy: ## Synchronise go.mod / go.sum avec les imports (dépendances du projet)
	$(DC_RUN) go mod tidy

tests: ## Tests (race + couverture) — un seul package : make tests PKG=./internal/rules/
ifeq ($(PKG),)
	$(DC_RUN) go test -race -cover ./...
else
	$(DC_RUN) go test -race -cover $(PKG)
endif

tests-collector: ## Tests du package collector
	$(DC_RUN) go test -race -cover ./internal/collector/

tests-analyzer: ## Tests du package analyzer
	$(DC_RUN) go test -race -cover ./internal/analyzer/

tests-rules: ## Tests du package rules
	$(DC_RUN) go test -race -cover ./internal/rules/

check: ## Tout vérifier d'un coup : lint + vet + tests
	$(DC_RUN) bash -ec 'golangci-lint fmt --diff && golangci-lint run && go vet ./... && go test -race -cover ./...'

build: ## Compile bin/phperf et bin/phperf-ci
	$(DC_RUN) bash -ec 'go build -o bin/phperf ./cmd/phperf && go build -o bin/phperf-ci ./cmd/phperf-ci'

shell: ## Shell interactif dans le conteneur dev (go, golangci-lint…)
	$(DC_RUN) bash

up: ## Démarre l'interface web sur http://localhost:8080
	$(COMPOSE) up -d --build web
	@echo "Interface web : http://localhost:$${PHPERF_PORT:-8080}"

down: ## Arrête les conteneurs
	$(COMPOSE) down

logs: ## Affiche les logs du service web
	$(COMPOSE) logs -f web

clean: ## Supprime bin/ et les volumes de cache Go
	$(COMPOSE) down --volumes
	rm -rf bin
