.SILENT:
.PHONY: all delete-cluster create-cluster cert-manager namespace apply-cert-manager-yaml create-root-ca cert-manager-istio-csr \
        label-crds istioctl-install istioctl-version helm-install addons-install kiali-install \
        gateway-api istio-base apply-otel-telemetry create-demo-app build-images build-local-image \
        deploy-app label-ambient enroll-waypoint label-use-waypoint \
        build-load build-ad build-cart build-currency build-email build-payment build-recommendation build-productcatalog \
        build-all-go-services clean-builder-cache

# Tools
HELM := helm
KUBECTL := kubectl
KIND := kind
MAKE := make
ROLL ?= false

# Cluster configuration
CLUSTER_NAME := demo
KIND_CONFIG  := ~/kind-with-istio/kind-config.yaml

all: delete-cluster create-cluster cert-manager namespace apply-cert-manager-yaml \
     create-root-ca cert-manager-istio-csr label-crds istioctl-install istioctl-version \
     helm-install addons-install kiali-install gateway-api istio-base create-demo-app \
     apply-otel-telemetry build-local-image deploy-app label-ambient enroll-waypoint label-use-waypoint
	@echo "✔ Full environment setup completed"

#  1. Delete existing Kind cluster
delete-cluster:
	@echo "[1/23] Deleting existing Kind cluster '$(CLUSTER_NAME)'..."
	@$(KIND) delete cluster --name $(CLUSTER_NAME) || echo "  (no existing cluster)"

#  2. Create Kind cluster
create-cluster:
	@echo "[2/23] Creating Kind cluster '$(CLUSTER_NAME)'..."
	@$(KIND) create cluster --config $(KIND_CONFIG) --name $(CLUSTER_NAME)

#  3. Install or upgrade cert-manager
cert-manager:
	@echo "[3/23] Installing or upgrading cert-manager..."
	@$(HELM) repo add jetstack https://charts.jetstack.io --force-update
	@$(HELM) upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager \
		--create-namespace \
		--version v1.16.1 \
		--set crds.enabled=true

#  4. Create istio-system namespace
namespace:
	@echo "[4/23] Creating 'istio-system' namespace..."
	@$(KUBECTL) create namespace istio-system 2>/dev/null || echo "  namespace already exists"

#  5. Apply cert-manager custom resources
apply-cert-manager-yaml:
	@echo "[5/23] Applying cert-manager custom resources..."
	@$(KUBECTL) apply -f ./release/cert-manager.yaml
	@echo "  → Waiting for Certificate to be ready..."
	@$(KUBECTL) wait --for=condition=Ready certificate/istio-ca -n istio-system --timeout=120s

#  6. Create istio-root-ca secret
create-root-ca: apply-cert-manager-yaml
	@echo "[6/23] Creating/updating 'istio-root-ca' secret..."
	@$(KUBECTL) get -n istio-system secret istio-ca -o go-template='{{index .data "tls.crt"}}' | base64 -d > ca.pem
	@$(KUBECTL) create secret generic -n cert-manager istio-root-ca --from-file=ca.pem=ca.pem

#  7. Install or upgrade cert-manager-istio-csr
cert-manager-istio-csr: create-root-ca
	@echo "[7/23] Installing or upgrading cert-manager-istio-csr..."
	@$(HELM) upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
		--install \
		--version=0.12.0 \
		--wait \
		--namespace cert-manager \
		--set "app.tls.rootCAFile=/var/run/secrets/istio-csr/ca.pem" \
		--set "app.tls.certificateDNSNames={cert-manager-istio-csr.cert-manager.svc,istio-csr.cert-manager.svc}" \
		--set "app.server.caTrustedNodeAccounts=istio-system/ztunnel" \
		--set "volumeMounts[0].name=root-ca" \
		--set "volumeMounts[0].mountPath=/var/run/secrets/istio-csr" \
		--set "volumes[0].name=root-ca" \
		--set "volumes[0].secret.secretName=istio-root-ca"

#  8. Label Istio CRDs for Helm management
label-crds:
	@echo "[8/23] Labeling Istio CRDs..."
	@for crd in $$($(KUBECTL) get crds -l chart=istio -o name) $$($(KUBECTL) get crds -l app.kubernetes.io/part-of=istio -o name); do \
		$(KUBECTL) label $$crd app.kubernetes.io/managed-by=Helm --overwrite; \
		$(KUBECTL) annotate $$crd meta.helm.sh/release-name=istio-base --overwrite; \
		$(KUBECTL) annotate $$crd meta.helm.sh/release-namespace=istio-system --overwrite; \
	done

#  9. Download istioctl
istioctl-install:
	@echo "[9/23] Downloading istioctl..."
	@curl -sL https://istio.io/downloadIstioctl | sh -
	@export PATH=$HOME/.istioctl/bin:$$PATH

# 10. Verify istioctl version
istioctl-version:
	@echo "[10/23] istioctl version:"
	@istioctl version

# 11. Install Helm3 and add Kiali repo
helm-install:
	@echo "[11/23] Installing Helm3 and adding Kiali repo..."
	@curl -sL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
	@$(HELM) repo add kiali https://kiali.org/helm-charts
	@$(HELM) repo update

# 12. Install Istio addons (Prometheus, Grafana, Jaeger)
addons-install:
	@echo "[12/23] Installing Istio addons..."
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/addons/prometheus.yaml
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/addons/grafana.yaml
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/addons/jaeger.yaml

# 13. Install Kiali
kiali-install:
	@echo "[13/23] Installing Kiali..."
	@$(HELM) install kiali-server kiali/kiali-server \
		--namespace istio-system \
		--set deployment.image.version="v2.8" \
		--set auth.strategy="anonymous" \
		--set external_services.istio.root_namespace="istio-system" \
		--set external_services.custom_dashboards.enabled=true \
		--set external_services.prometheus.url="http://prometheus.istio-system:9090/" \
		--set external_services.tracing.enabled=true \
		--set external_services.tracing.internal_url="http://tracing.istio-system:16685/jaeger" \
		--set external_services.tracing.use_grpc=true \
		--set external_services.tracing.external_url="http://localhost:16686/jaeger" \
		--set external_services.grafana.enabled=true \
		--set external_services.grafana.internal_url="http://grafana.istio-system:3000/" \
		--set external_services.grafana.external_url="http://localhost:3000/grafana"

# 14. Ensure Gateway API CRDs
gateway-api:
	@echo "[14/23] Ensuring Gateway API CRDs..."
	@$(KUBECTL) get crd gateways.gateway.networking.k8s.io &>/dev/null || \
		$(KUBECTL) kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.2.1" | $(KUBECTL) apply -f -

# 15. Install or upgrade Istio core components (base, istiod, cni, ztunnel)
istio-base:
	@echo "[15/23] Installing or upgrading Istio core components..."
	@$(HELM) upgrade --install istio-base istio/base \
		--namespace istio-system \
		--version 1.26.0 \
		--wait
	@$(HELM) upgrade --install istiod istio/istiod \
		--namespace istio-system \
		--version 1.26.0 \
		--set profile=ambient \
		--set pilot.env.ENABLE_CA_SERVER=false \
		--set global.caAddress=cert-manager-istio-csr.cert-manager.svc:443 \
		--set global.meshID=cluster.local \
		--values ./release/istio-meshconfig-extras.yaml \
		--wait
	@$(HELM) upgrade --install istio-cni istio/cni \
		--namespace istio-system \
		--version 1.26.0 \
		--set profile=ambient \
		--wait
	@$(HELM) upgrade --install ztunnel istio/ztunnel \
		--namespace istio-system \
		--version=1.26.0 \
		--set "caAddress=cert-manager-istio-csr.cert-manager.svc:443" \
		--wait

# 16. Apply otel-collector and telemetry manifests
apply-otel-telemetry:
	@echo "[16/23] Applying OTel Collector and Telemetry manifests..."
	@$(KUBECTL) apply -f ./release/otel-collector.yaml
	@$(KUBECTL) apply -f ./release/als-telemetry.yaml
	@if [ "$(ROLL)" = "true" ]; then \
		echo "[20/23] Restarting deployments in 'observability' namespace..."; \
		$(KUBECTL) rollout restart deployment -n observability; \
	fi

# 17. Create demo-app namespace
create-demo-app:
	@echo "[17/23] Creating 'demo-app' namespace..."
	@$(KUBECTL) create namespace demo-app 2>/dev/null || echo "  namespace already exists"

# 18. Build images in parallel
SERVICES := adservice cartservice checkoutservice currencyservice \
            emailservice frontend paymentservice productcatalogservice \
            recommendationservice shippingservice

SERVICE_PORT_adservice=9555
SERVICE_PORT_cartservice=7070
SERVICE_PORT_checkoutservice=5050
SERVICE_PORT_currencyservice=7000
SERVICE_PORT_emailservice=8080
SERVICE_PORT_frontend=8080
SERVICE_PORT_paymentservice=50051
SERVICE_PORT_productcatalogservice=3550
SERVICE_PORT_recommendationservice=8080
SERVICE_PORT_shippingservice=50051

define BUILD_SERVICE
  @echo "  - Building $(1) image..."
  docker build \
    --build-arg SERVICE_NAME=$(1) \
    --build-arg SERVICE_PORT=$$(SERVICE_PORT_$(1)) \
    -t $(1):local \
    -f Dockerfile.service .
endef

build-images:
	@echo "Building service images in parallel..."
	@$(MAKE) -j 1 $(addprefix build-,$(SERVICES))

build-%:
	$(call BUILD_SERVICE,$*)

clean-builder-cache:
	@echo "Cleaning old Docker builder cache (older than 24h)..."
	docker builder prune --force --filter "until=24h"

build-loadgenerator:
	@echo "    - Building k6-loadgenerator image..."
	cd src/k6-loadgenerator && docker build -t k6-loadgenerator:local .

KIND_LOAD_IMAGES := frontend k6-loadgenerator adservice checkoutservice cartservice \
               currencyservice emailservice paymentservice recommendationservice \
               productcatalogservice shippingservice

# 19. Load into kind and prune
build-local-image: build-images build-loadgenerator clean-builder-cache
	@echo "→ Loading images into kind cluster..."
	@for svc in $(KIND_LOAD_IMAGES); do \
	  echo "   • $$svc:local → kind load"; \
	  $(KIND) load docker-image $$svc:local --name $(CLUSTER_NAME); \
	done

# 20. Deploy application manifests
deploy-app: create-demo-app build-local-image
	@echo "[20/23] Deploying application manifests to 'demo-app'..."
	@$(KUBECTL) apply -f ./release/kubernetes-manifests.yaml -n demo-app
	@if [ "$(ROLL)" = "true" ]; then \
		echo "[20/23] Restarting deployments in 'demo-app' namespace..."; \
		$(KUBECTL) rollout restart deployment -n demo-app; \
	fi

# 21. Label demo-app namespace for ambient mode
label-ambient: create-demo-app
	@echo "[21/23] Labeling 'demo-app' namespace for ambient mode..."
	@$(KUBECTL) label namespace demo-app istio.io/dataplane-mode=ambient --overwrite

# 22. Enroll demo-app namespace with Waypoint
enroll-waypoint: label-ambient gateway-api
	@echo "[22/23] Enrolling 'demo-app' namespace with Istio Waypoint..."
	@istioctl waypoint apply -n demo-app --enroll-namespace

# 23. Label demo-app to use Waypoint
label-use-waypoint: enroll-waypoint
	@echo "[23/23] Labeling 'demo-app' namespace to use Waypoint..."
	@$(KUBECTL) label namespace demo-app istio.io/use-waypoint=waypoint --overwrite
