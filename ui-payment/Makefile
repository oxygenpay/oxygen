.PHONY: help

include .env

help: ## List of commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | sed 's/Makefile//' | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

precommit-ts: ## Add pre-commit hook
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

lint-es: ## List js
	@set -eu; \
	echo 'Linting via ESLint'; \
	NODE_OPTIONS="--max-old-space-size=5000" npx eslint src --ext .ts,.tsx,.js

lint: ## Run linter
	make -j3 verify-ts lint-es

dev: ## Run dev
	npx vite --host pay-local.sandbox.o2pay.co

build: ## Build dist
	npx tsc && npx vite build --base=${VITE_ROOTPATH}

preview: ## Run Vite preview
	npx vite preview

prepare: ## Prepare
	npx husky install
