#!/bin/bash
# kube-services-deploy.sh - Script to deploy TradingLab event system to Kubernetes

set -e  # Exit on error

# Environment variables
export REGISTRY="us-central1-docker.pkg.dev/financetracker-451021/tradinglab"
export VERSION=$(git describe --tags --always --dirty || echo "dev")
export NAMESPACE="tradinglab"

# Make sure namespace exists
kubectl get namespace $NAMESPACE > /dev/null 2>&1 || kubectl create namespace $NAMESPACE

# Apply ConfigMap for event configuration
echo "Deploying event configuration..."
kubectl apply -f kube/nats/nats-deployment.yaml

# Create credentials secret if it doesn't exist
if ! kubectl get secret tradinglab-credentials -n $NAMESPACE > /dev/null 2>&1; then
    echo "Creating credentials secret..."

    # Create secret
    kubectl create secret generic tradinglab-credentials \
        --namespace $NAMESPACE \
        --from-file=kube/market-data/alpaca_secret.yaml

    echo "Secret created."
else
    echo "Credentials secret already exists."
fi

# Deploy NATS server
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
envsubst < kube/ui/api-gateway-deployment.yaml | kubectl apply -f -

echo "Deploying tradinglab service..."
envsubst < kube/tradinglab/tradinglab-server.yaml | kubectl apply -f -

echo "Deploying UI..."
envsubst < kube/ui/ui-deployment.yaml | kubectl apply -f -

# Wait for deployments to be ready
echo "Waiting for deployments to be ready..."
for component in event-client market-data-service event-hub api-gateway tradinglab-service tradinglab-ui; do
    kubectl rollout status deployment/$component -n $NAMESPACE --timeout=120s
done

echo "Deployment completed successfully!"
echo "You can check the status of your deployments with:"
echo "kubectl get pods -n $NAMESPACE"