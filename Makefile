# Makefile for API Gateway project

# Variables
KUSTOMIZE_VERSION := 5.0.0
KUBECTL_VERSION := 1.28.0
K9S_VERSION := 0.27.4

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

.PHONY: help install-tools validate build deploy clean test docs

# Default target
help: ## Show this help message
	@echo "API Gateway Project - Available commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m%-15s\033[0m %s\n", "Target", "Description"} /^[a-zA-Z_-]+:.*?##/ { printf "\033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

install-tools: ## Install required tools (kustomize, k9s, etc.)
	@echo "$(YELLOW)Installing tools...$(NC)"
	@if ! command -v kustomize >/dev/null 2>&1; then \
		echo "Installing Kustomize..."; \
		curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash; \
		sudo mv kustomize /usr/local/bin/; \
	fi
	@if ! command -v k9s >/dev/null 2>&1; then \
		echo "Installing k9s..."; \
		brew install k9s || (echo "Please install k9s manually" && exit 1); \
	fi
	@echo "$(GREEN)Tools installed successfully!$(NC)"

validate: ## Validate Kubernetes manifests
	@echo "$(YELLOW)Validating Kubernetes manifests...$(NC)"
	@echo "Validating development overlay..."
	@kustomize build api-gateway/overlays/development > /tmp/dev-manifest.yaml
	@echo "Validating production overlay..."
	@kustomize build api-gateway/overlays/production > /tmp/prod-manifest.yaml
	@echo "$(GREEN)Validation completed!$(NC)"

build: validate ## Build Kubernetes manifests
	@echo "$(YELLOW)Building manifests...$(NC)"
	@mkdir -p build/manifests
	@kustomize build api-gateway/overlays/development > build/manifests/development.yaml
	@kustomize build api-gateway/overlays/production > build/manifests/production.yaml
	@echo "$(GREEN)Manifests built in build/manifests/$(NC)"

deploy-dev: build ## Deploy to development environment
	@echo "$(YELLOW)Deploying to development...$(NC)"
	@kubectl apply -k api-gateway/overlays/development
	@echo "$(GREEN)Development deployment completed!$(NC)"
	@echo "Access URLs:"
	@echo "  Frontend: http://localhost:30080/frontend"
	@echo "  Backend:  http://localhost:30080/"

deploy-prod: build ## Deploy to production environment
	@echo "$(YELLOW)Deploying to production...$(NC)"
	@read -p "Are you sure you want to deploy to production? [y/N] " confirm && [ "$$confirm" = "y" ]
	@kubectl apply -k api-gateway/overlays/production
	@echo "$(GREEN)Production deployment completed!$(NC)"

deploy-argocd: ## Deploy ArgoCD applications
	@echo "$(YELLOW)Deploying ArgoCD applications...$(NC)"
	@kubectl apply -f argocd/projects/api-gateway-project.yaml
	@kubectl apply -f bootstrap/root-app.yaml
	@echo "$(GREEN)ArgoCD applications deployed!$(NC)"

clean: ## Clean up deployments
	@echo "$(YELLOW)Cleaning up...$(NC)"
	@kubectl delete -k api-gateway/overlays/development --ignore-not-found=true
	@kubectl delete -k api-gateway/overlays/production --ignore-not-found=true
	@rm -rf build/
	@echo "$(GREEN)Cleanup completed!$(NC)"

test: ## Run tests and validations
	@echo "$(YELLOW)Running tests...$(NC)"
	@make validate
	@echo "Testing frontend HTML..."
	@if [ -f "api-gateway/base/frontend/index.html" ]; then \
		echo "HTML file exists ✓"; \
	else \
		echo "$(RED)HTML file missing!$(NC)"; exit 1; \
	fi
	@echo "Testing nginx config..."
	@if [ -f "api-gateway/base/frontend/nginx.conf" ]; then \
		echo "Nginx config exists ✓"; \
	else \
		echo "$(RED)Nginx config missing!$(NC)"; exit 1; \
	fi
	@echo "$(GREEN)All tests passed!$(NC)"

docs: ## Generate documentation
	@echo "$(YELLOW)Generating documentation...$(NC)"
	@mkdir -p docs
	@echo "# API Gateway Resources" > docs/resources.md
	@echo "" >> docs/resources.md
	@echo "## Development Environment" >> docs/resources.md
	@kustomize build api-gateway/overlays/development | grep -E "^(kind:|  name:)" >> docs/resources.md
	@echo "" >> docs/resources.md
	@echo "## Production Environment" >> docs/resources.md
	@kustomize build api-gateway/overlays/production | grep -E "^(kind:|  name:)" >> docs/resources.md
	@echo "$(GREEN)Documentation generated in docs/$(NC)"

status: ## Show deployment status
	@echo "$(YELLOW)Deployment Status:$(NC)"
	@echo ""
	@echo "Pods:"
	@kubectl get pods -l project=api-gateway --all-namespaces 2>/dev/null || echo "No pods found"
	@echo ""
	@echo "Services:"
	@kubectl get services -l project=api-gateway --all-namespaces 2>/dev/null || echo "No services found"
	@echo ""
	@echo "HTTPRoutes:"
	@kubectl get httproutes -l project=api-gateway --all-namespaces 2>/dev/null || echo "No HTTPRoutes found"

k9s: ## Launch k9s for cluster management
	@echo "$(YELLOW)Launching k9s...$(NC)"
	@k9s

logs: ## Show logs for all project pods
	@echo "$(YELLOW)Recent logs for API Gateway pods:$(NC)"
	@kubectl logs -l project=api-gateway --all-namespaces --tail=50 2>/dev/null || echo "No pods found"

port-forward: ## Set up port forwarding
	@echo "$(YELLOW)Setting up port forwarding...$(NC)"
	@echo "Forwarding Envoy Gateway NodePort..."
	@kubectl port-forward -n envoy-gateway-system service/envoy-gateway-nodeport 8080:80 &
	@echo "$(GREEN)Access the application at http://localhost:8080$(NC)"
	@echo "Press Ctrl+C to stop port forwarding"

install-precommit: ## Install pre-commit hooks
	@echo "$(YELLOW)Installing pre-commit hooks...$(NC)"
	@pip install pre-commit || echo "Please install pip first"
	@pre-commit install
	@echo "$(GREEN)Pre-commit hooks installed!$(NC)"

# Development helpers
dev-setup: install-tools install-precommit ## Full development setup
	@echo "$(GREEN)Development environment ready!$(NC)"

quick-deploy: deploy-dev status ## Quick development deployment with status
	@echo "$(GREEN)Quick deployment completed!$(NC)"
