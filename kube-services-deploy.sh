#!/bin/bash
# kube-services-deploy.sh - Script to deploy TradingLab event system to Kubernetes

set -e  # Exit on error

# Environment variables
export REGISTRY="us-central1-docker.pkg.dev/financetracker-451021/tradinglab"
export VERSION=$(git describe --tags --always || echo "dev")
# Remove the "dirty" suffix which causes image pull issues
export NAMESPACE="tradinglab"

# Make sure namespace exists
kubectl get namespace $NAMESPACE > /dev/null 2>&1 || kubectl create namespace $NAMESPACE

# Create credentials secrets if they don't exist
if ! kubectl get secret alpaca-credentials -n $NAMESPACE > /dev/null 2>&1; then
    echo "Creating Alpaca credentials secret..."
    kubectl apply -f kube/market-data/alpaca_secret.yaml
    echo "Alpaca secret created."
else
    echo "Alpaca credentials secret already exists."
fi

if ! kubectl get secret alpha-vantage-credentials -n $NAMESPACE > /dev/null 2>&1; then
    echo "Creating Alpha Vantage credentials secret..."
    kubectl apply -f kube/market-data/alpha_vantage_key.yaml
    echo "Alpha Vantage secret created."
else
    echo "Alpha Vantage credentials secret already exists."
fi

# Create gcr-json-key secret if it doesn't exist
if ! kubectl get secret gcr-json-key -n $NAMESPACE > /dev/null 2>&1; then
    echo "Creating gcr-json-key secret..."
    # Check if credentials file exists
    if [ -f "$HOME/.config/gcloud/application_default_credentials.json" ]; then
        kubectl create secret docker-registry gcr-json-key \
            --namespace $NAMESPACE \
            --docker-server=us-central1-docker.pkg.dev \
            --docker-username=_json_key \
            --docker-password="$(cat $HOME/.config/gcloud/application_default_credentials.json)" \
            --docker-email=$(gcloud config get-value account)
        echo "GCR secret created."
    else
        echo "WARNING: GCP credentials file not found. You need to create gcr-json-key secret manually."
        echo "Run: gcloud auth application-default login"
    fi
else
    echo "GCR secret already exists."
fi

# Deploy NATS server (only once)
echo "Deploying NATS server..."
envsubst < kube/nats/nats-deployment.yaml | kubectl apply -f -

# Wait for NATS to be ready
echo "Waiting for NATS server to be ready..."
kubectl rollout status statefulset/nats -n $NAMESPACE --timeout=120s

# Deploy event components
echo "Deploying event client..."
envsubst < kube/event-client/event-client.yaml | kubectl apply -f -

echo "Deploying market data service..."
envsubst < kube/market-data/market-data.yaml | kubectl apply -f -

echo "Deploying event hub..."
envsubst < kube/event-hub/event-hub.yaml | kubectl apply -f -

# Deploy API components
echo "Deploying API gateway..."
envsubst < kube/api-gateway/deployment.yaml | kubectl apply -f -

echo "Deploying tradinglab service..."
envsubst < kube/tradinglab/tradinglab-server.yaml | kubectl apply -f -

echo "Deploying UI..."
envsubst < kube/ui/ui-deployment.yaml | kubectl apply -f -

# Deploy ingress if exists
if [ -f kube/ingress.yaml ]; then
    echo "Deploying ingress..."
    envsubst < kube/ingress.yaml | kubectl apply -f -
fi

# Wait for deployments to be ready
echo "Waiting for deployments to be ready..."
for component in event-client market-data-service event-hub api-gateway tradinglab-service tradinglab-ui; do
    kubectl rollout status deployment/$component -n $NAMESPACE --timeout=120s || echo "Warning: $component deployment not ready in time"
done

echo "Deployment completed successfully!"
echo "You can check the status of your deployments with:"
echo "kubectl get pods -n $NAMESPACE"