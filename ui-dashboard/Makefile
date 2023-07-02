.PHONY: help

include .env

help: ## List of commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

precommit-ts: ## Precommit TS
	NODE_OPTIONS="--max-old-space-size=5000" npx tsc \
		--noEmit \
		--incremental \
		--skipLibCheck \
		--project ./tsconfig.json;

verify-ts: ## Verify TS
	@set -eu; status=0; \
	echo 'Verifying src/'; \
	NODE_OPTIONS="--max-old-space-size=5000" npx tsc --noEmit --skipLibCheck -p ./tsconfig.json || status=1; \
	echo 'Verification finished'; \
	exit $$status;

lint-es: ## Lint ES
	@set -eu; \
	echo 'Linting via ESLint'; \
	NODE_OPTIONS="--max-old-space-size=5000" npx eslint src --ext .ts,.tsx,.js

lint: ## Lint
	make -j3 verify-ts lint-es

dev: ## Run dev
	npx vite --host app-local.sandbox.o2pay.co

build: ## Build dist
	npx tsc && npx vite build --base=${VITE_ROOTPATH}

preview: ## Preview
	npx vite preview

prepare: ## Prepare
	npx husky install