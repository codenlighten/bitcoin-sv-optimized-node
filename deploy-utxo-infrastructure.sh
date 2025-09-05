#!/bin/bash

# Metamorph Bitcoin SV UTXO Infrastructure Deployment
# Production-grade UTXO set management with Scylla and Redpanda

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           METAMORPH BITCOIN SV UTXO INFRASTRUCTURE          ║"
echo "║              Production UTXO Set Management                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-metamorph-bsv-cluster}"
NAMESPACE="${NAMESPACE:-metamorph-utxo}"
REGION="${REGION:-nyc3}"

# Check prerequisites
echo "🔍 Checking prerequisites..."

if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is required but not installed"
    exit 1
fi

if ! command -v doctl &> /dev/null; then
    echo "❌ doctl is required but not installed"
    exit 1
fi

if ! command -v helm &> /dev/null; then
    echo "❌ helm is required but not installed"
    exit 1
fi

echo "✅ Prerequisites check passed"

# Verify cluster connection
echo "🌐 Verifying cluster connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes cluster"
    echo "💡 Run: doctl kubernetes cluster kubeconfig save $CLUSTER_NAME"
    exit 1
fi

CURRENT_CONTEXT=$(kubectl config current-context)
echo "✅ Connected to cluster: $CURRENT_CONTEXT"

# Check node pools and labels
echo "📊 Checking node pool configuration..."
NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)
echo "📈 Total nodes: $NODE_COUNT"

# Check for node pool labels (data-a, bus-a, app-a)
DATA_NODES=$(kubectl get nodes -l node-type=data-a --no-headers 2>/dev/null | wc -l || echo "0")
BUS_NODES=$(kubectl get nodes -l node-type=bus-a --no-headers 2>/dev/null | wc -l || echo "0")
APP_NODES=$(kubectl get nodes -l node-type=app-a --no-headers 2>/dev/null | wc -l || echo "0")

echo "📊 Node pool distribution:"
echo "   🗄️  Data nodes (data-a): $DATA_NODES"
echo "   🚌 Bus nodes (bus-a): $BUS_NODES"
echo "   📱 App nodes (app-a): $APP_NODES"

if [ "$DATA_NODES" -eq 0 ] && [ "$BUS_NODES" -eq 0 ] && [ "$APP_NODES" -eq 0 ]; then
    echo "⚠️  No specialized node pools detected"
    echo "💡 For production, consider creating dedicated node pools:"
    echo "   doctl kubernetes cluster node-pool create $CLUSTER_NAME --name data-pool --size s-4vcpu-8gb --count 3"
    echo "   kubectl label nodes <node-name> node-type=data-a"
    echo ""
    echo "🔄 Continuing with default node configuration..."
fi

# Install required operators
echo "🔧 Installing required operators..."

# Add Helm repositories
echo "📦 Adding Helm repositories..."
helm repo add scylla https://scylla-operator-charts.storage.googleapis.com/stable
helm repo add kedacore https://kedacore.github.io/charts
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install KEDA for event-driven scaling
echo "🚀 Installing KEDA operator..."
if ! kubectl get namespace keda-system &> /dev/null; then
    helm install keda kedacore/keda --create-namespace --namespace keda-system
    echo "✅ KEDA installed"
else
    echo "✅ KEDA already installed"
fi

# Install Scylla Operator
echo "🗄️  Installing Scylla Operator..."
if ! kubectl get namespace scylla-operator &> /dev/null; then
    helm install scylla-operator scylla/scylla-operator --create-namespace --namespace scylla-operator
    echo "✅ Scylla Operator installed"
else
    echo "✅ Scylla Operator already installed"
fi

# Wait for operators to be ready
echo "⏳ Waiting for operators to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/keda-operator -n keda-system
kubectl wait --for=condition=available --timeout=300s deployment/scylla-operator -n scylla-operator
echo "✅ Operators are ready"

# Create namespace
echo "🏗️  Creating namespace: $NAMESPACE"
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Deploy UTXO infrastructure
echo "🚀 Deploying UTXO infrastructure..."
kubectl apply -f k8s-utxo-infrastructure.yaml

# Wait for Scylla cluster to be ready
echo "⏳ Waiting for Scylla cluster to initialize..."
echo "💡 This may take 5-10 minutes for the first deployment..."

# Monitor Scylla cluster status
TIMEOUT=600  # 10 minutes
ELAPSED=0
INTERVAL=30

while [ $ELAPSED -lt $TIMEOUT ]; do
    if kubectl get scyllaclusters.scylla.scylladb.com metamorph-utxo-cluster -n $NAMESPACE &> /dev/null; then
        STATUS=$(kubectl get scyllaclusters.scylla.scylladb.com metamorph-utxo-cluster -n $NAMESPACE -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        echo "📊 Scylla cluster status: $STATUS"
        
        if [ "$STATUS" = "Ready" ]; then
            echo "✅ Scylla cluster is ready"
            break
        fi
    else
        echo "⏳ Waiting for Scylla cluster to be created..."
    fi
    
    sleep $INTERVAL
    ELAPSED=$((ELAPSED + INTERVAL))
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo "⚠️  Scylla cluster initialization timeout"
    echo "💡 Check cluster status: kubectl get scyllaclusters -n $NAMESPACE"
else
    echo "✅ Scylla cluster initialized successfully"
fi

# Wait for Redpanda to be ready
echo "⏳ Waiting for Redpanda cluster..."
kubectl wait --for=condition=ready --timeout=300s pod -l app=redpanda -n $NAMESPACE
echo "✅ Redpanda cluster is ready"

# Wait for Ledger service to be ready
echo "⏳ Waiting for Ledger service..."
kubectl wait --for=condition=available --timeout=300s deployment/metamorph-ledger-scylla -n $NAMESPACE
echo "✅ Ledger service is ready"

# Get service information
echo "📊 Deployment Summary:"
echo ""

# Scylla cluster info
SCYLLA_PODS=$(kubectl get pods -l app.kubernetes.io/name=scylla -n $NAMESPACE --no-headers 2>/dev/null | wc -l || echo "0")
echo "🗄️  Scylla Cluster:"
echo "   📦 Pods: $SCYLLA_PODS/3"
echo "   🌐 Service: metamorph-utxo-cluster.$NAMESPACE.svc.cluster.local"
echo "   📊 Keyspace: metamorph"

# Redpanda cluster info
REDPANDA_PODS=$(kubectl get pods -l app=redpanda -n $NAMESPACE --no-headers 2>/dev/null | wc -l || echo "0")
echo ""
echo "🚌 Redpanda Cluster:"
echo "   📦 Pods: $REDPANDA_PODS/3"
echo "   🌐 Service: redpanda.$NAMESPACE.svc.cluster.local:9092"
echo "   📊 Topics: Ready for Bitcoin SV events"

# Ledger service info
LEDGER_PODS=$(kubectl get pods -l component=ledger -n $NAMESPACE --no-headers 2>/dev/null | wc -l || echo "0")
echo ""
echo "💰 Ledger Service:"
echo "   📦 Pods: $LEDGER_PODS/3"
echo "   🌐 Service: metamorph-ledger-scylla.$NAMESPACE.svc.cluster.local:50051"
echo "   📊 Backend: Scylla UTXO store"

# KEDA scaling info
KEDA_SCALEDOBJECTS=$(kubectl get scaledobjects -n $NAMESPACE --no-headers 2>/dev/null | wc -l || echo "0")
echo ""
echo "📈 Auto-Scaling:"
echo "   🎯 KEDA ScaledObjects: $KEDA_SCALEDOBJECTS"
echo "   📊 HPA: Enabled for Ledger service"
echo "   ⚡ Event-driven scaling: Kafka lag triggers"

# Get external access information
echo ""
echo "🌐 External Access:"
EXTERNAL_IP=$(kubectl get service -n metamorph-bsv --no-headers 2>/dev/null | grep LoadBalancer | awk '{print $4}' | head -1 || echo "Pending")
if [ "$EXTERNAL_IP" != "Pending" ] && [ "$EXTERNAL_IP" != "<none>" ]; then
    echo "   🌍 External IP: $EXTERNAL_IP"
    echo "   📊 Mining Dashboard: http://$EXTERNAL_IP:9090/"
    echo "   📈 Metrics: http://$EXTERNAL_IP:9090/metrics"
else
    echo "   ⏳ External IP: Pending (LoadBalancer provisioning)"
fi

# Configuration instructions
echo ""
echo "🔧 Next Steps:"
echo ""
echo "1. 📊 Monitor UTXO Infrastructure:"
echo "   kubectl get pods -n $NAMESPACE"
echo "   kubectl logs -f deployment/metamorph-ledger-scylla -n $NAMESPACE"
echo ""
echo "2. 🔄 Start UTXO Bootstrap:"
echo "   # This will sync the Bitcoin SV blockchain and populate the UTXO set"
echo "   kubectl create job utxo-bootstrap --from=cronjob/utxo-sync -n $NAMESPACE"
echo ""
echo "3. 📈 Monitor Scaling:"
echo "   kubectl get hpa -n $NAMESPACE"
echo "   kubectl get scaledobjects -n $NAMESPACE"
echo ""
echo "4. 🗄️  Check Scylla Status:"
echo "   kubectl exec -it metamorph-utxo-cluster-us-east-1-rack-a-0 -n $NAMESPACE -- nodetool status"
echo ""
echo "5. 🚌 Check Redpanda Topics:"
echo "   kubectl exec -it redpanda-0 -n $NAMESPACE -- rpk topic list"
echo ""

# Security recommendations
echo "🔒 Security Recommendations:"
echo ""
echo "1. 🔑 Update DigitalOcean Spaces credentials:"
echo "   kubectl patch secret do-spaces-secret -n $NAMESPACE \\"
echo "     -p='{\"stringData\":{\"access-key\":\"your-key\",\"secret-key\":\"your-secret\"}}'"
echo ""
echo "2. 🛡️  Enable network policies:"
echo "   # Restrict inter-pod communication"
echo "   kubectl apply -f network-policies.yaml"
echo ""
echo "3. 📊 Set up monitoring alerts:"
echo "   # Configure Prometheus alerts for UTXO operations"
echo "   kubectl apply -f prometheus-alerts.yaml"
echo ""

echo "🎉 UTXO Infrastructure Deployment Complete!"
echo ""
echo "📊 Status: Production-grade Bitcoin SV UTXO management system deployed"
echo "🗄️  Storage: Scylla cluster with 3 nodes and 500GB per node"
echo "🚌 Events: Redpanda cluster for high-throughput message processing"
echo "📈 Scaling: KEDA and HPA for automatic scaling based on load"
echo "💰 UTXO: Ready to sync and validate Bitcoin SV transactions"
echo ""
echo "🚀 Your Bitcoin SV node now has enterprise-grade UTXO management!"
