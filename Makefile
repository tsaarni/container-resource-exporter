.PHONY: all lint lint-go clean container kind-create kind-setup kind-load kind-delete help

KIND_CLUSTER_NAME ?= container-resource-exporter

all:
	go build .

container: ## Create container image.
	docker buildx build -t ghcr.io/tsaarni/container-resource-exporter:latest .

clean: ## Clean up build artifacts.
	@rm -f container-resource-exporter

lint: lint-go ## Run all linters.

lint-go: ## Run golangci-lint.
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run

kind-load: ## Load container image into kind cluster and restart daemonset.
	kind load docker-image ghcr.io/tsaarni/container-resource-exporter:latest --name $(KIND_CLUSTER_NAME)
	kubectl delete pod -l app=container-resource-exporter

kind-create: ## Create kind cluster.
	kind create cluster --name $(KIND_CLUSTER_NAME) --config examples/contour/configs/kind.yaml

kind-setup: ## Setup Contour and observability stack.
	./examples/contour/setup.sh

kind-delete: ## Delete kind cluster.
	kind delete cluster --name $(KIND_CLUSTER_NAME)

help: ## Show this help.
	@awk '/^[a-zA-Z_-]+:.*## / { sub(/:.*## /, "\t"); split($$0,a, "\t"); printf "\033[36m%-30s\033[0m %s\n", a[1], a[2] }' $(MAKEFILE_LIST)
