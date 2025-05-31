.SILENT:
.PHONY: all from-addons delete-cluster create-cluster cert-manager namespace apply-cert-manager-yaml create-root-ca cert-manager-istio-csr label-crds istioctl-install istioctl-version addons-install helm-install kiali-install gateway-api create-demo-app apply-otel-telemetry istio-base create-demo-app build-loadgenerator deploy-app label-ambient enroll-waypoint label-use-waypoint recreate-cluster reapply-istio

# Tools
HELM := helm
KUBECTL := kubectl
KIND := kind

# Cluster configuration
CLUSTER_NAME := demo
KIND_CONFIG  := ~/kind-with-istio/kind-config.yaml

from-addons: addons-install helm-install kiali-install gateway-api create-demo-app apply-otel-telemetry istio-base restart-istio istio-cni ztunnel create-demo-app build-loadgenerator deploy-app label-ambient enroll-waypoint label-use-waypoint

# all: delete existing cluster, create cluster, then full Istio setup
all: delete-cluster create-cluster cert-manager namespace apply-cert-manager-yaml create-root-ca cert-manager-istio-csr label-crds istioctl-install istioctl-version addons-install helm-install kiali-install gateway-api create-demo-app apply-otel-telemetry istio-base create-demo-app build-loadgenerator deploy-app label-ambient enroll-waypoint label-use-waypoint
	@echo "✔ Full environment setup completed"

# クラスターを削除せずに再生成するためのレシピ
recreate-cluster:
	@echo "[1/26] Recreating Kind cluster '$(CLUSTER_NAME)'..."
	@$(KIND) create cluster --config $(KIND_CONFIG) --name $(CLUSTER_NAME) || true
	@echo "✔ Cluster recreation completed"

# 既存のクラスターを保持したまま、Istioの設定を再適用
reapply-istio: recreate-cluster cert-manager namespace apply-cert-manager-yaml create-root-ca cert-manager-istio-csr label-crds istioctl-install istioctl-version istio-operator-install addons-install helm-install kiali-install gateway-api create-demo-app apply-otel-telemetry istio-base istiod restart-istio istio-cni ztunnel create-demo-app build-loadgenerator deploy-app label-ambient enroll-waypoint label-use-waypoint
	@echo "✔ Istio configuration reapplied"

# 1. Delete existing Kind cluster
delete-cluster:
	@echo "[1/26] Deleting existing Kind cluster '$(CLUSTER_NAME)'..."
	@$(KIND) delete cluster --name $(CLUSTER_NAME) || echo "  (no existing cluster)"

# 2. Create Kind cluster
create-cluster:
	@echo "[2/26] Creating Kind cluster '$(CLUSTER_NAME)'..."
	@$(KIND) create cluster --config $(KIND_CONFIG) --name $(CLUSTER_NAME)

# 3. Install or upgrade cert-manager
cert-manager:
	@echo "[3/26] Installing or upgrading cert-manager..."
	@$(HELM) repo add jetstack https://charts.jetstack.io --force-update
	@$(HELM) upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager \
		--create-namespace \
		--version v1.16.1 \
		--set crds.enabled=true

# 4. Create istio-system namespace
namespace:
	@echo "[4/26] Creating 'istio-system' namespace..."
	@$(KUBECTL) create namespace istio-system 2>/dev/null || echo "  namespace already exists"

# 5. Apply cert-manager custom resources
apply-cert-manager-yaml:
	@echo "[5/26] Applying cert-manager custom resources..."
	@$(KUBECTL) apply -f ./release/cert-manager.yaml

# 6. Create istio-root-ca secret
create-root-ca:
	@echo "[6/26] Creating/updating 'istio-root-ca' secret..."
	@$(KUBECTL) get secret istio-ca -n istio-system \
		-o go-template='{{index .data "tls.crt"}}' \
	| base64 -d \
	| $(KUBECTL) create secret generic istio-root-ca \
		-n cert-manager \
		--from-file=ca.pem=/dev/stdin \
		--dry-run=client -o yaml \
	| $(KUBECTL) apply -f -

# 7. Install or upgrade cert-manager-istio-csr
cert-manager-istio-csr:
	@echo "[7/26] Installing or upgrading cert-manager-istio-csr..."
	@$(HELM) upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
		--install \
		--version=0.12.0 \
		--namespace cert-manager \
		--set "app.tls.rootCAFile=/var/run/secrets/istio-csr/ca.pem" \
		--set "app.tls.certificateDNSNames={cert-manager-istio-csr.cert-manager.svc,istio-csr.cert-manager.svc}" \
		--set "app.server.caTrustedNodeAccounts=istio-system/ztunnel" \
		--set "volumeMounts[0].name=root-ca" \
		--set "volumeMounts[0].mountPath=/var/run/secrets/istio-csr" \
		--set "volumes[0].name=root-ca" \
		--set "volumes[0].secret.secretName=istio-root-ca"

# 8. Label Istio CRDs for Helm management
label-crds:
	@echo "[8/26] Labeling Istio CRDs..."
	@for crd in $$($(KUBECTL) get crds -l chart=istio -o name) $$($(KUBECTL) get crds -l app.kubernetes.io/part-of=istio -o name); do \
		$(KUBECTL) label $$crd app.kubernetes.io/managed-by=Helm --overwrite; \
		$(KUBECTL) annotate $$crd meta.helm.sh/release-name=istio-base --overwrite; \
		$(KUBECTL) annotate $$crd meta.helm.sh/release-namespace=istio-system --overwrite; \
	done

# 9. Download istioctl
istioctl-install:
	@echo "[9/26] Downloading istioctl..."
	@curl -sL https://istio.io/downloadIstioctl | sh -
	@export PATH=$HOME/.istioctl/bin:$PATH

# 10. Verify istioctl version
istioctl-version:
	@echo "[10/26] istioctl version:"
	@istioctl version

# 12. Install Istio addons (Prometheus, Grafana, Jaeger)
addons-install:
	@echo "[12/26] Installing Istio addons..."
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.25/samples/addons/prometheus.yaml
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.25/samples/addons/grafana.yaml
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.25/samples/addons/jaeger.yaml
	# @$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/addons/loki.yaml -n istio-system
	# @$(KUBECTL) apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/open-telemetry/loki/otel.yaml -n istio-system

# 13. Install Helm3 and add Kiali repo
helm-install:
	@echo "[13/26] Installing Helm3 and adding Kiali repo..."
	@curl -sL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
	@$(HELM) repo add kiali https://kiali.org/helm-charts
	@$(HELM) repo update

# 14. Install Kiali
kiali-install:
	@echo "[14/26] Installing Kiali..."
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

# 15. Ensure Gateway API CRDs
gateway-api:
	@echo "[15/26] Ensuring Gateway API CRDs..."
	@$(KUBECTL) get crd gateways.gateway.networking.k8s.io &>/dev/null || \
		$(KUBECTL) kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.2.1" | $(KUBECTL) apply -f -

# 16. Install or upgrade Istio core components (base, istiod, cni, ztunnel)
istio-base:
	@echo "[16/26] Installing or upgrading Istio core components..."
	@$(HELM) upgrade --install istio-base istio/base \
		--namespace istio-system \
		--version 1.24.0 \
		--wait
	@$(HELM) upgrade --install istiod istio/istiod \
		--namespace istio-system \
		--version 1.24.0 \
		--set profile=ambient \
		--set pilot.env.ENABLE_CA_SERVER=false \
		--set global.caAddress=cert-manager-istio-csr.cert-manager.svc:443 \
		--set global.meshID=cluster.local \
		--values ./release/istio-meshconfig-extras.yaml \
		--wait
	@$(HELM) upgrade --install istio-cni istio/cni \
		--namespace istio-system \
		--version 1.24.0 \
		--set profile=ambient \
		--wait
	@$(HELM) upgrade --install ztunnel istio/ztunnel \
		--namespace istio-system \
		--version=1.24.0 \
		--set "caAddress=cert-manager-istio-csr.cert-manager.svc:443" \
		--wait

# 21. Create demo-app namespace
create-demo-app:
	@echo "[21/26] Creating 'demo-app' namespace..."
	@$(KUBECTL) create namespace demo-app 2>/dev/null || echo "  namespace already exists"

# 22. Build and load loadgenerator image
build-loadgenerator:
	@echo "[22/26] Building and loading loadgenerator image..."
	@cd ./src/k6-loadgenerator && docker build -t k6-loadgenerator:local .
	@$(KIND) load docker-image k6-loadgenerator:local --name $(CLUSTER_NAME)

# 23. Deploy application manifests
deploy-app: create-demo-app build-loadgenerator
	@echo "[23/26] Deploying application manifests to 'demo-app'..."
	@$(KUBECTL) apply -f ./release/kubernetes-manifests.yaml -n demo-app

# 24. Label demo-app namespace for ambient mode
label-ambient: create-demo-app
	@echo "[24/26] Labeling 'demo-app' namespace for ambient mode..."
	@$(KUBECTL) label namespace demo-app istio.io/dataplane-mode=ambient --overwrite

# 25. Enroll demo-app namespace with Waypoint
enroll-waypoint: label-ambient gateway-api
	@echo "[25/26] Enrolling 'demo-app' namespace with Istio Waypoint..."
	@istioctl waypoint apply -n demo-app --enroll-namespace

# 26. Label demo-app to use Waypoint
label-use-waypoint: enroll-waypoint
	@echo "[26/26] Labeling 'demo-app' namespace to use Waypoint..."
	@$(KUBECTL) label namespace demo-app istio.io/use-waypoint=waypoint --overwrite

# 27. Apply otel-collector and telemetry manifests
apply-otel-telemetry: istio-base
	@$(KUBECTL) apply -f ./release/otel-collector.yaml
	@$(KUBECTL) apply -f ./release/telemetry.yaml