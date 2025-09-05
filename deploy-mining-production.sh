#!/bin/bash

# Bitcoin SV Mining Node - Production Deployment Script
# Deploys complete Metamorph Bitcoin SV mining node to DigitalOcean with live mining capabilities

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           METAMORPH BITCOIN SV MINING NODE DEPLOYMENT       ║"
echo "║              Production Mining Operations Setup              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
CLUSTER_NAME="metamorph-mining-production"
REGION="nyc1"
NODE_SIZE="s-4vcpu-8gb"
NODE_COUNT=3
NAMESPACE="metamorph-mining"

# Check prerequisites
echo "🔍 Checking deployment prerequisites..."

# Check for required environment variables
if [ -z "$DIGITAL_OCEAN_TOKEN" ]; then
    echo "❌ Error: DIGITAL_OCEAN_TOKEN not set"
    echo "💡 Please set your DigitalOcean API token:"
    echo "   export DIGITAL_OCEAN_TOKEN=your_token_here"
    exit 1
fi

# Check for doctl
if ! command -v doctl &> /dev/null; then
    echo "❌ Error: doctl CLI not found"
    echo "💡 Please install doctl: https://docs.digitalocean.com/reference/doctl/how-to/install/"
    exit 1
fi

# Check for kubectl
if ! command -v kubectl &> /dev/null; then
    echo "❌ Error: kubectl not found"
    echo "💡 Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

echo "✅ Prerequisites check completed"
echo ""

# Authenticate with DigitalOcean
echo "🔐 Authenticating with DigitalOcean..."
doctl auth init --access-token "$DIGITAL_OCEAN_TOKEN"
echo "✅ DigitalOcean authentication successful"
echo ""

# Create or update Kubernetes cluster
echo "🏗️  Setting up Kubernetes cluster for Bitcoin SV mining..."
if doctl kubernetes cluster get "$CLUSTER_NAME" &> /dev/null; then
    echo "📋 Cluster $CLUSTER_NAME already exists, updating..."
    doctl kubernetes cluster upgrade "$CLUSTER_NAME" --latest
else
    echo "🆕 Creating new Kubernetes cluster: $CLUSTER_NAME"
    doctl kubernetes cluster create "$CLUSTER_NAME" \
        --region "$REGION" \
        --size "$NODE_SIZE" \
        --count "$NODE_COUNT" \
        --auto-upgrade \
        --surge-upgrade \
        --maintenance-window "saturday=02:00" \
        --tag "metamorph,bitcoin-sv,mining,production" \
        --wait
fi

# Configure kubectl
echo "⚙️  Configuring kubectl for mining cluster..."
doctl kubernetes cluster kubeconfig save "$CLUSTER_NAME"
kubectl config use-context "do-$REGION-$CLUSTER_NAME"
echo "✅ kubectl configured for $CLUSTER_NAME"
echo ""

# Create namespace
echo "📁 Creating Kubernetes namespace: $NAMESPACE"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Create production configuration
echo "⚙️  Creating production mining configuration..."
cat > k8s-mining-production.yaml << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: metamorph-mining-config
  namespace: metamorph-mining
data:
  BITCOIN_LIVE_MODE: "true"
  BITCOIN_NETWORK: "testnet"
  BITCOIN_MAX_WORKERS: "8"
  BITCOIN_MIN_WORKERS: "2"
  BITCOIN_MAX_PEERS: "16"
  BITCOIN_MIN_PEERS: "8"
  LOG_LEVEL: "info"
  ENABLE_METRICS: "true"
  METRICS_PORT: "9090"
  PORTAL_PORT: "8080"
  LEDGER_GRPC_PORT: "50051"
  ENGINE_GRPC_PORT: "50052"
---
apiVersion: v1
kind: Secret
metadata:
  name: metamorph-mining-secrets
  namespace: metamorph-mining
type: Opaque
stringData:
  BITCOIN_MINING_ADDRESS: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
  BITCOIN_POOL_URL: ""
  BITCOIN_POOL_USERNAME: ""
  BITCOIN_POOL_PASSWORD: ""
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metamorph-mining-node
  namespace: metamorph-mining
  labels:
    app: metamorph-mining
    component: mining-node
spec:
  replicas: 2
  selector:
    matchLabels:
      app: metamorph-mining
      component: mining-node
  template:
    metadata:
      labels:
        app: metamorph-mining
        component: mining-node
    spec:
      containers:
      - name: mining-node
        image: golang:1.21-alpine
        command: ["/bin/sh"]
        args:
          - -c
          - |
            apk add --no-cache git build-base
            git clone https://github.com/codenlighten/bitcoin-sv-optimized-node.git /app
            cd /app
            go mod tidy
            go run ./cmd/mining-demo/main.go
        envFrom:
        - configMapRef:
            name: metamorph-mining-config
        - secretRef:
            name: metamorph-mining-secrets
        ports:
        - containerPort: 8080
          name: portal
        - containerPort: 9090
          name: metrics
        - containerPort: 50051
          name: ledger-grpc
        - containerPort: 50052
          name: engine-grpc
        - containerPort: 8333
          name: bitcoin-p2p
        resources:
          requests:
            cpu: 2000m
            memory: 4Gi
          limits:
            cpu: 4000m
            memory: 8Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 60
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: metamorph-mining-service
  namespace: metamorph-mining
  labels:
    app: metamorph-mining
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
    name: portal
  - port: 9090
    targetPort: 9090
    name: metrics
  - port: 50051
    targetPort: 50051
    name: ledger-grpc
  - port: 50052
    targetPort: 50052
    name: engine-grpc
  - port: 8333
    targetPort: 8333
    name: bitcoin-p2p
  selector:
    app: metamorph-mining
    component: mining-node
---
apiVersion: v1
kind: Service
metadata:
  name: metamorph-mining-metrics
  namespace: metamorph-mining
  labels:
    app: metamorph-mining
    component: metrics
spec:
  ports:
  - port: 9090
    targetPort: 9090
    name: metrics
  selector:
    app: metamorph-mining
    component: mining-node
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: metamorph-mining-hpa
  namespace: metamorph-mining
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: metamorph-mining-node
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
EOF

# Deploy to Kubernetes
echo "🚀 Deploying Bitcoin SV mining node to production..."
kubectl apply -f k8s-mining-production.yaml

# Wait for deployment
echo "⏳ Waiting for mining node deployment to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/metamorph-mining-node -n "$NAMESPACE"

# Get service information
echo "📊 Getting service information..."
kubectl get services -n "$NAMESPACE"

# Get LoadBalancer IP
echo "🌐 Waiting for LoadBalancer IP assignment..."
EXTERNAL_IP=""
while [ -z "$EXTERNAL_IP" ]; do
    echo "Waiting for external IP..."
    EXTERNAL_IP=$(kubectl get svc metamorph-mining-service -n "$NAMESPACE" --template="{{range .status.loadBalancer.ingress}}{{.ip}}{{end}}")
    [ -z "$EXTERNAL_IP" ] && sleep 10
done

echo ""
echo "🎉 Bitcoin SV Mining Node Deployment Successful!"
echo ""
echo "📋 Production Mining Node Information:"
echo "   🌐 External IP: $EXTERNAL_IP"
echo "   🚪 Portal API: http://$EXTERNAL_IP/"
echo "   📊 Metrics: http://$EXTERNAL_IP:9090/metrics"
echo "   🔧 Ledger gRPC: $EXTERNAL_IP:50051"
echo "   ⚙️  Engine gRPC: $EXTERNAL_IP:50052"
echo "   ⛏️  Bitcoin P2P: $EXTERNAL_IP:8333"
echo ""
echo "⛏️  Mining Configuration:"
echo "   🌐 Network: Bitcoin SV Testnet (LIVE mode)"
echo "   👷 Workers: 2-8 (auto-scaling based on load)"
echo "   🏊 Pool Mode: Solo mining (configure pool for pool mining)"
echo "   📈 Scaling: Auto-scaling enabled (2-10 replicas)"
echo ""
echo "📊 Monitoring Commands:"
echo "   kubectl get pods -n $NAMESPACE"
echo "   kubectl logs -f deployment/metamorph-mining-node -n $NAMESPACE"
echo "   kubectl describe hpa metamorph-mining-hpa -n $NAMESPACE"
echo ""
echo "🔧 Configuration Updates:"
echo "   kubectl edit configmap metamorph-mining-config -n $NAMESPACE"
echo "   kubectl edit secret metamorph-mining-secrets -n $NAMESPACE"
echo ""
echo "⛏️  To enable pool mining, update the secrets:"
echo "   kubectl patch secret metamorph-mining-secrets -n $NAMESPACE -p='{\"stringData\":{\"BITCOIN_POOL_URL\":\"stratum+tcp://pool.example.com:4444\"}}'"
echo "   kubectl patch secret metamorph-mining-secrets -n $NAMESPACE -p='{\"stringData\":{\"BITCOIN_POOL_USERNAME\":\"your_username\"}}'"
echo "   kubectl patch secret metamorph-mining-secrets -n $NAMESPACE -p='{\"stringData\":{\"BITCOIN_POOL_PASSWORD\":\"your_password\"}}'"
echo ""
echo "🎯 Your Bitcoin SV mining node is now operational and ready to mine!"

# Save deployment information
cat > deployment-info.txt << EOF
Metamorph Bitcoin SV Mining Node - Production Deployment
Deployed: $(date)
Cluster: $CLUSTER_NAME
Region: $REGION
Namespace: $NAMESPACE
External IP: $EXTERNAL_IP

Services:
- Portal API: http://$EXTERNAL_IP/
- Metrics: http://$EXTERNAL_IP:9090/metrics
- Ledger gRPC: $EXTERNAL_IP:50051
- Engine gRPC: $EXTERNAL_IP:50052
- Bitcoin P2P: $EXTERNAL_IP:8333

Mining Configuration:
- Network: Bitcoin SV Testnet (LIVE mode)
- Workers: Auto-scaling 2-10 replicas
- Mode: Solo mining (configurable for pool mining)

Management:
- kubectl get pods -n $NAMESPACE
- kubectl logs -f deployment/metamorph-mining-node -n $NAMESPACE
EOF

echo "💾 Deployment information saved to deployment-info.txt"
echo ""
echo "✨ Bitcoin SV mining node deployment completed successfully!"
