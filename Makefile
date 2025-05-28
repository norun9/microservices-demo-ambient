.SILENT:
.PHONY: all label-crds cert-manager namespace apply-cert-manager-yaml extract-ca create-root-ca cert-manager-istio-csr add-helm-repos istio-base istiod restart-istio istio-cni ztunnel gateway-api

# Helm と kubectl のパスを必要に応じて変更
HELM := helm
KUBECTL := kubectl

all: label-crds cert-manager namespace apply-cert-manager-yaml extract-ca create-root-ca cert-manager-istio-csr add-helm-repos istio-base istiod restart-istio istio-cni ztunnel gateway-api
	@echo "✔ Istio ambient environment setup completed"

# 1. CRD に Helm 管理ラベル・アノテーションを追加
label-crds:
	@echo "[1/15] Labeling Istio CRDs..."
	@for crd in $$($(KUBECTL) get crds -l chart=istio -o name) $$($(KUBECTL) get crds -l app.kubernetes.io/part-of=istio -o name); do \
	  $(KUBECTL) label $$crd app.kubernetes.io/managed-by=Helm --overwrite; \
	  $(KUBECTL) annotate $$crd meta.helm.sh/release-name=istio-base --overwrite; \
	  $(KUBECTL) annotate $$crd meta.helm.sh/release-namespace=istio-system --overwrite; \
	done

# 2. cert-manager のインストール
cert-manager:
	@echo "[2/15] Installing or upgrading cert-manager..."
	@$(HELM) repo add jetstack https://charts.jetstack.io --force-update
	@$(HELM) upgrade --install cert-manager jetstack/cert-manager \
	  --namespace cert-manager \
	  --create-namespace \
	  --version v1.16.1 \
	  --set crds.enabled=true

# 3. istio-system 名前空間作成
namespace:
	@echo "[3/15] Creating istio-system namespace..."
	@$(KUBECTL) create namespace istio-system 2>/dev/null || echo "  namespace already exists"

# 4. cert-manager 用追加 YAML の適用
apply-cert-manager-yaml:
	@echo "[4/15] Applying cert-manager custom resources..."
	@$(KUBECTL) apply -f ./release/cert-manager.yaml

# 5. CA 証明書の抽出
extract-ca:
	@echo "[5/15] Extracting Istio CA certificate..."
	@$(KUBECTL) get -n istio-system secret istio-ca -ogo-template='{{index .data "tls.crt"}}' | base64 -d > ca.pem

# 6. istio-root-ca シークレット作成
create-root-ca:
	@echo "[6/15] Inspecting ca.pem content with OpenSSL..."
	@openssl x509 -in ca.pem -noout -text
	@echo "[7/15] Creating/updating istio-root-ca Secret in cert-manager..."
	@$(KUBECTL) create secret generic -n cert-manager istio-root-ca \
	  --from-file=ca.pem=ca.pem \
	  --dry-run=client -o yaml \
	  | $(KUBECTL) apply -f -

# 7. cert-manager-istio-csr のインストール
cert-manager-istio-csr:
	@echo "[8/15] Installing or upgrading cert-manager-istio-csr..."
	@$(HELM) repo add jetstack https://charts.jetstack.io --force-update
	@$(HELM) upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
	  --install \
	  --version=0.12.0 \
	  --namespace cert-manager \
	  --wait \
	  --set "app.tls.rootCAFile=/var/run/secrets/istio-csr/ca.pem" \
	  --set "app.tls.certificateDNSNames={cert-manager-istio-csr.cert-manager.svc,istio-csr.cert-manager.svc}" \
	  --set "app.server.caTrustedNodeAccounts=istio-system/ztunnel" \
	  --set "volumeMounts[0].name=root-ca" \
	  --set "volumeMounts[0].mountPath=/var/run/secrets/istio-csr" \
	  --set "volumes[0].name=root-ca" \
	  --set "volumes[0].secret.secretName=istio-root-ca"

# 8. Istio Helm リポジトリ追加
add-helm-repos:
	@echo "[9/15] Adding Istio Helm repo..."
	@$(HELM) repo add istio https://istio-release.storage.googleapis.com/charts
	@$(HELM) repo update

# 9. istio-base のインストール
istio-base:
	@echo "[10/15] Checking if istio-base is already installed..."
	@if ! $(HELM) status istio-base -n istio-system &>/dev/null; then \
		echo "Installing istio-base..."; \
		$(HELM) install istio-base istio/base \
		  --namespace istio-system \
		  --version 1.24.0 \
		  --wait; \
	else \
		echo "istio-base is already installed, skipping..."; \
	fi

# 10. istiod のインストール／アップグレード
istiod:
	@echo "[11/15] Installing/upgrading istiod..."
	@$(HELM) upgrade --install istiod istio/istiod \
	  --namespace istio-system \
	  --version 1.24.0 \
	  --set profile=ambient \
	  --set pilot.env.ENABLE_CA_SERVER=false \
	  --set global.caAddress=cert-manager-istio-csr.cert-manager.svc:443 \
	  --set global.meshID=cluster.local \
	  --values ./release/istio-meshconfig-extras.yaml \
	  --wait

# 11. istiod & ztunnel の再起動
restart-istio:
	@echo "[12/15] Restarting istiod and ztunnel..."
	@$(KUBECTL) -n istio-system rollout restart deployment istiod
	@$(KUBECTL) -n istio-system rollout restart daemonset ztunnel

# 12. istio-cni のインストール
istio-cni:
	@echo "[13/15] Checking if istio-cni is already installed..."
	@if ! $(HELM) status istio-cni -n istio-system &>/dev/null; then \
		echo "Installing istio-cni..."; \
		$(HELM) install istio-cni istio/cni \
		  --namespace istio-system \
		  --version 1.24.0 \
		  --set profile=ambient \
		  --wait; \
	else \
		echo "istio-cni is already installed, skipping..."; \
	fi

# 13. ztunnel のインストール
ztunnel:
	@echo "[14/15] Checking if ztunnel is already installed..."
	@if ! $(HELM) status ztunnel -n istio-system &>/dev/null; then \
		echo "Installing ztunnel..."; \
		$(HELM) install ztunnel istio/ztunnel \
		  --namespace istio-system \
		  --version 1.24.0 \
		  --set caAddress=cert-manager-istio-csr.cert-manager.svc:443 \
		  --wait; \
	else \
		echo "ztunnel is already installed, skipping..."; \
	fi

# 14. Gateway API CRD の適用
gateway-api:
	@echo "[15/15] Ensuring Gateway API CRDs..."
	@$(KUBECTL) get crd gateways.gateway.networking.k8s.io &>/dev/null || \
	  $(KUBECTL) apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml
