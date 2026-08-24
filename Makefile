COMPOSE := docker compose
DC_RUN  := $(COMPOSE) run --rm app

.DEFAULT_GOAL := help
.PHONY: help fix lint vet tidy tests tests-collector tests-analyzer tests-rules tests-scorer tests-baseline tests-storage tests-report tests-ui ci-demo demo-collect e2e check build shell up down logs clean

help: ## Affiche cette aide
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

fix: ## Corrige formatage + auto-fixables (best effort : n'échoue pas sur ce qui demande une correction manuelle)
	$(DC_RUN) bash -ec 'golangci-lint fmt && (golangci-lint run --fix || true)'

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

tests-scorer: ## Tests du package scorer
	$(DC_RUN) go test -race -cover ./internal/scorer/

tests-baseline: ## Tests du package baseline
	$(DC_RUN) go test -race -cover ./internal/baseline/

tests-storage: ## Tests du package storage
	$(DC_RUN) go test -race -cover ./internal/storage/

tests-report: ## Tests du package report
	$(DC_RUN) go test -race -cover ./internal/report/

tests-ui: ## Tests du package ui
	$(DC_RUN) go test -race -cover ./internal/ui/

ci-demo: ## Démo CI : baseline puis run sur la fixture nplus1 (exit 0 attendu)
	$(DC_RUN) bash -ec 'go build -buildvcs=false -o bin/phperf-ci ./cmd/phperf-ci \
		&& bin/phperf-ci baseline --profile=scripts/fixtures/nplus1.json --rules=proto/rules.example.yaml \
		&& bin/phperf-ci run --profile=scripts/fixtures/nplus1.json --rules=proto/rules.example.yaml'

demo-collect: ## Démo collecte réelle : profile le scénario PHP de démo (image php+xhprof) → bin/phperf-demo.json
	docker build -q -t phperf-demo-php -f scripts/php/Dockerfile.demo scripts/php
	docker run --rm -v $$PWD:/work -w /work phperf-demo-php \
		php scripts/php/phperf-profile.php --output=bin/phperf-demo.json scripts/fixtures/php-demo/scenario.php
	@echo "Profil : bin/phperf-demo.json — visualiser avec : make up PHPERF_PROFILE=bin/phperf-demo.json"

e2e: ## Test E2E collecte→CI→UI (côté hôte, requiert Docker + réseau ; ~2 min à froid)
	sh scripts/e2e.sh

check: ## Tout vérifier d'un coup : lint + vet + tests
	$(DC_RUN) bash -ec 'golangci-lint fmt --diff && golangci-lint run && go vet ./... && go test -race -cover ./...'

build: ## Compile bin/phperf et bin/phperf-ci
	$(DC_RUN) bash -ec 'go build -buildvcs=false -o bin/phperf ./cmd/phperf && go build -buildvcs=false -o bin/phperf-ci ./cmd/phperf-ci'

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
