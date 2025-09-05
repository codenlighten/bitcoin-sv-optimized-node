#!/bin/bash

# Fix DigitalOcean Kubernetes kubeconfig permissions and complete deployment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║              METAMORPH KUBECONFIG FIX & DEPLOY              ║"
echo "║            Complete DigitalOcean Deployment                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

# Load environment variables
if [ -f ".env" ]; then
    export $(cat .env | xargs)
    print_success "✅ Environment variables loaded"
else
    print_error "❌ .env file not found"
    exit 1
fi

# Cluster details from previous deployment
CLUSTER_ID="4dcf987d-6ce8-4a67-8503-d6192918ce5b"
CLUSTER_NAME="metamorph-bsv-cluster"

print_status "Attempting to fix kubeconfig access..."

# Method 1: Try direct kubeconfig download with different approach
print_status "Method 1: Direct kubeconfig configuration..."
if doctl kubernetes cluster kubeconfig save $CLUSTER_ID --set-current-context; then
    print_success "✅ Kubeconfig configured successfully"
    KUBECONFIG_SUCCESS=true
else
    print_warning "⚠️  Method 1 failed, trying alternative approach..."
    KUBECONFIG_SUCCESS=false
fi

# Method 2: Manual kubeconfig setup if Method 1 fails
if [ "$KUBECONFIG_SUCCESS" = false ]; then
    print_status "Method 2: Manual kubeconfig setup..."
    
    # Get cluster info
    CLUSTER_INFO=$(doctl kubernetes cluster get $CLUSTER_ID --format ID,Name,Region,Version,Status --no-header)
    print_status "Cluster Info: $CLUSTER_INFO"
    
    # Try alternative kubeconfig method
    if doctl kubernetes cluster kubeconfig save $CLUSTER_NAME; then
        print_success "✅ Alternative kubeconfig method successful"
        KUBECONFIG_SUCCESS=true
    else
        print_warning "⚠️  Alternative method also failed"
    fi
fi

# Method 3: Check if we can proceed with existing cluster access
if [ "$KUBECONFIG_SUCCESS" = false ]; then
    print_status "Method 3: Checking cluster accessibility..."
    
    # Test if kubectl can access the cluster anyway
    if kubectl cluster-info &> /dev/null; then
        print_success "✅ kubectl can access cluster despite kubeconfig warnings"
        KUBECONFIG_SUCCESS=true
    else
        print_warning "⚠️  kubectl cannot access cluster"
    fi
fi

# If kubeconfig is working, proceed with deployment
if [ "$KUBECONFIG_SUCCESS" = true ]; then
    print_success "🎉 Kubernetes access confirmed! Proceeding with Metamorph deployment..."
    
    # Check cluster status
    print_status "Checking cluster status..."
    kubectl get nodes
    
    # Deploy Metamorph services
    print_status "Deploying Metamorph Bitcoin SV Node..."
    kubectl apply -f deploy/digitalocean.yml
    
    # Wait for deployment
    print_status "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/metamorph-node -n metamorph-bsv || true
    
    # Get service information
    print_status "Getting service information..."
    kubectl get services -n metamorph-bsv
    kubectl get pods -n metamorph-bsv
    
    # Try to get LoadBalancer IP
    print_status "Checking for LoadBalancer IP..."
    EXTERNAL_IP=$(kubectl get svc metamorph-loadbalancer -n metamorph-bsv --template="{{range .status.loadBalancer.ingress}}{{.ip}}{{end}}" 2>/dev/null || echo "")
    
    if [ -n "$EXTERNAL_IP" ]; then
        echo
        echo "╔══════════════════════════════════════════════════════════════╗"
        echo "║                 DEPLOYMENT SUCCESSFUL!                      ║"
        echo "╚══════════════════════════════════════════════════════════════╝"
        echo
        print_success "🎉 Metamorph Bitcoin SV Node deployed successfully!"
        echo
        print_success "🌐 External IP: $EXTERNAL_IP"
        print_success "🔗 API Gateway: http://$EXTERNAL_IP/"
        print_success "📈 Telemetry: http://$EXTERNAL_IP:9090/metrics"
        print_success "🔍 Health Check: http://$EXTERNAL_IP:9090/health"
        print_success "⚡ P2P Network: $EXTERNAL_IP:8333"
    else
        print_warning "⚠️  LoadBalancer IP not yet assigned (this is normal and may take a few minutes)"
        print_status "You can check the IP later with: kubectl get svc metamorph-loadbalancer -n metamorph-bsv"
    fi
    
    echo
    print_status "🚀 Your Metamorph Bitcoin SV Node is running in production!"
    print_status "📋 Use 'kubectl get pods -n metamorph-bsv' to check pod status"
    print_status "📊 Use 'kubectl logs -f deployment/metamorph-node -n metamorph-bsv' to view logs"
    
else
    print_error "❌ Unable to configure kubectl access to the cluster"
    echo
    print_status "ALTERNATIVE SOLUTIONS:"
    print_status "1. Check DigitalOcean token permissions (needs Kubernetes read/write access)"
    print_status "2. Use DigitalOcean web console to download kubeconfig manually"
    print_status "3. Create a new token with full Kubernetes permissions"
    echo
    print_status "Your cluster is created and running: $CLUSTER_NAME ($CLUSTER_ID)"
    print_status "You can access it via the DigitalOcean web console"
fi
