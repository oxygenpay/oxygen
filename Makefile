.PHONY: help

VERSION=$(shell git describe --tags --dirty --always)
COMMIT=$(shell git rev-parse HEAD)
EMBED_FRONTEND ?= 1

# The -w turns off DWARF debugging information
# The -s turns off generation of the Go symbol table
# The -X adds a string value definition of the form importpath.name=value
LDFLAGS=-ldflags "-w -s -X 'main.gitVersion=${VERSION}' -X 'main.gitCommit=${COMMIT}' -X 'main.embedFrontend=${EMBED_FRONTEND}'"

PKG_LIST := $(shell go list ./... | grep -v pkg/api- | grep -v internal/db | tr "\n" " ")

help: ## List of commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

lint: ## Lint code using golangci-lint
	golangci-lint run -v ./...

codegen: ## Generate stuff for Oxygen :)
	sqlc generate -f scripts/sqlc.yaml

	# dashboard api
	mkdir -p "pkg/api-dashboard/v1"
	swagger flatten api/proto/merchant/dashboard-v1.yml -o web/redoc/dashboard/v1/dashboard.json
	swagger generate model -f web/redoc/dashboard/v1/dashboard.json -t pkg/api-dashboard/v1 -m model

	# internal admin api
	mkdir -p "pkg/api-admin/v1"
	swagger flatten api/proto/admin/admin-v1.yml -o web/redoc/admin/v1/admin.json
	swagger generate model -f web/redoc/admin/v1/admin.json -t pkg/api-admin/v1 -m model

	# merchant api
	mkdir -p "pkg/api-merchant/v1"
	swagger flatten api/proto/merchant/merchant-v1.yml -o web/redoc/merchant/v1/merchant.json
	swagger generate model -f web/redoc/merchant/v1/merchant.json -t pkg/api-merchant/v1 -m model

	# kms api & client
	mkdir -p "pkg/api-kms/v1"
	swagger flatten api/proto/kms/kms-v1.yml -o web/redoc/kms/v1/kms.json
	swagger generate client -f web/redoc/kms/v1/kms.json -t pkg/api-kms/v1 -m model

	# public facing payment api
	mkdir -p "pkg/api-payment/v1"
	swagger flatten api/proto/payment/payment-v1.yml -o web/redoc/payment/v1/payment.json
	swagger generate model -f web/redoc/payment/v1/payment.json -t pkg/api-payment/v1 -m model

mock: ## Generate mocks (not included in codegen command)
	mockery --dir pkg/api-kms/v1 --all --output pkg/api-kms/v1/mock --outpkg mock
	mockery --dir internal/service/processing --all --output internal/test/mock/ --outpkg mock

build: ## Build app
	go build ${LDFLAGS} -o bin/oxygen main.go

run: ## Run application (without building)
	./bin/oxygen serve-web --config=$$(pwd)/config/oxygen.yml

run-kms: ## Run KMS (without building)
	./bin/oxygen serve-kms --config=$$(pwd)/config/oxygen.yml

run-scheduler: ## Run Scheduler (without building)
	./bin/oxygen run-scheduler --config=$$(pwd)/config/oxygen.yml

local: codegen build run ## Build & Run App

local-kms: codegen build run-kms ## Build & Run KMS
local-scheduler: codegen build run-scheduler ## Build & Run Scheduler

test: ## Run tests with race detector
	@go test -race ${PKG_LIST}

require-deps: ## Require cli tools for development
	go install github.com/rubenv/sql-migrate/...@latest
	go install github.com/kyleconroy/sqlc/cmd/sqlc@latest
	go install github.com/cespare/reflex@latest
	go install github.com/vektra/mockery/v2@v2.32.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3
    # todo go-swagger as swagger

docker-build: ## Build docker image for Oxygen
	DOCKER_BUILDKIT=1 docker build -t oxygen-local .

docker-local:
	docker-compose -f docker-compose.local.yml up

clean-test-dbs: ## Drop test "oxygen_test*" databases
	psql -c "\l" | grep "oxygen_test" | awk '{print $$1}' | xargs -I {} psql -c "drop database {};"