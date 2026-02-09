.PHONY: help build up down restart logs clean test-fe test-be build-fe build-be push-fe push-be

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Docker commands
build: ## Build all Docker images
	docker-compose build

up: ## Start all services
	docker-compose up -d

down: ## Stop all services
	docker-compose down

restart: ## Restart all services
	docker-compose restart

logs: ## Show logs from all services
	docker-compose logs -f

logs-fe: ## Show frontend logs
	docker-compose logs -f frontend

logs-be: ## Show backend logs
	docker-compose logs -f backend

clean: ## Remove all containers and images
	docker-compose down -v --rmi all

# Development
dev: ## Start development environment
	docker-compose -f docker-compose.dev.yml up -d

dev-build: ## Build development images
	docker-compose -f docker-compose.dev.yml build

# Testing
test-fe: ## Run frontend tests
	cd excalidraw-fe && npm test

test-be: ## Run backend tests
	cd excalidraw-be && go test -v ./...

# Build individual services
build-fe: ## Build frontend image
	docker build -t excalidraw-fe:latest ./excalidraw-fe

build-be: ## Build backend image
	docker build -t excalidraw-be:latest ./excalidraw-be

# Production deployment
prod-up: ## Start production environment
	docker-compose up -d --build

prod-down: ## Stop production environment
	docker-compose down

# CI/CD helpers
lint-fe: ## Lint frontend code
	cd excalridraw-fe && npm run lint

lint-be: ## Lint backend code
	cd excalidraw-be && go fmt ./... && go vet ./...
