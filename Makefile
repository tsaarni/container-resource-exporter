.PHONY: all lint lint-go clean container kind-setup kind-create kind-load kind-delete help

all:
	go build .

container: ## Create container image.
	docker buildx build -t ghcr.io/tsaarni/container-resource-exporter:latest .

clean: ## Clean up build artifacts.
	@rm -f container-resource-exporter

lint: lint-go ## Run all linters.

lint-go: ## Run golangci-lint.
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1 run

kind-load: ## Load container image into kind cluster and restart daemonset.
	kind load docker-image ghcr.io/tsaarni/container-resource-exporter:latest --name container-resource-exporter
	kubectl rollout restart daemonset/container-resource-exporter

kind-create: ## Create kind cluster for e2e tests.
	kind create cluster --name container-resource-exporter --config=examples/kind/kind-cluster-config.yaml

kind-delete: ## Delete kind cluster used for e2e tests.
	kind delete cluster --name container-resource-exporter

help: ## Show this help.
	@awk '/^[a-zA-Z_-]+:.*## / { sub(/:.*## /, "\t"); split($$0,a, "\t"); printf "\033[36m%-30s\033[0m %s\n", a[1], a[2] }' $(MAKEFILE_LIST)
