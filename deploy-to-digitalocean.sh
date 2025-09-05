#!/bin/bash

# Metamorph Bitcoin SV Node - DigitalOcean Deployment Script
# Automated deployment to DigitalOcean Kubernetes

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
echo "║           METAMORPH DIGITALOCEAN DEPLOYMENT                 ║"
echo "║         Production Bitcoin SV Node Deployment               ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

# Check if .env file exists and load it
if [ -f ".env" ]; then
    print_status "Loading environment variables from .env..."
    export $(cat .env | xargs)
    if [ -z "$DIGITAL_OCEAN_TOKEN" ]; then
        print_error "DIGITAL_OCEAN_TOKEN not found in .env file"
        exit 1
    fi
    print_success "✅ DigitalOcean token loaded"
else
    print_error ".env file not found. Please create it with DIGITAL_OCEAN_TOKEN"
    exit 1
fi

# Check if doctl is installed
if ! command -v doctl &> /dev/null; then
    print_warning "doctl CLI not found. Installing..."
    
    # Install doctl
    cd ~
    wget https://github.com/digitalocean/doctl/releases/download/v1.104.0/doctl-1.104.0-linux-amd64.tar.gz
    tar xf doctl-1.104.0-linux-amd64.tar.gz
    sudo mv doctl /usr/local/bin
    rm doctl-1.104.0-linux-amd64.tar.gz
    
    print_success "✅ doctl installed"
fi

# Authenticate with DigitalOcean
print_status "Authenticating with DigitalOcean..."
doctl auth init --access-token $DIGITAL_OCEAN_TOKEN
print_success "✅ Authenticated with DigitalOcean"

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    print_warning "kubectl not found. Installing..."
    
    # Install kubectl
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/
    
    print_success "✅ kubectl installed"
fi

# Create or get Kubernetes cluster
CLUSTER_NAME="metamorph-bsv-cluster"
print_status "Checking for existing Kubernetes cluster..."

if doctl kubernetes cluster get $CLUSTER_NAME &> /dev/null; then
    print_success "✅ Using existing cluster: $CLUSTER_NAME"
else
    print_status "Creating new DigitalOcean Kubernetes cluster..."
    
    doctl kubernetes cluster create $CLUSTER_NAME \
        --region nyc1 \
        --version latest \
        --node-pool "name=metamorph-pool;size=s-4vcpu-8gb;count=3;auto-scale=true;min-nodes=3;max-nodes=10" \
        --wait
    
    print_success "✅ Kubernetes cluster created: $CLUSTER_NAME"
fi

# Configure kubectl
print_status "Configuring kubectl..."
doctl kubernetes cluster kubeconfig save $CLUSTER_NAME
print_success "✅ kubectl configured for cluster"

# Build and push Docker image to DigitalOcean Container Registry
print_status "Setting up DigitalOcean Container Registry..."

REGISTRY_NAME="metamorph-registry"
if ! doctl registry get $REGISTRY_NAME &> /dev/null; then
    print_status "Creating container registry..."
    doctl registry create $REGISTRY_NAME --region nyc3
    print_success "✅ Container registry created"
fi

# Login to registry
print_status "Logging into container registry..."
doctl registry login

# Build and tag image
print_status "Building and pushing Docker image..."
docker build -f Dockerfile.demo -t registry.digitalocean.com/$REGISTRY_NAME/metamorph-demo:latest .
docker push registry.digitalocean.com/$REGISTRY_NAME/metamorph-demo:latest
print_success "✅ Docker image pushed to registry"

# Update deployment YAML with correct image
print_status "Updating deployment configuration..."
sed -i "s|image: metamorph-demo:latest|image: registry.digitalocean.com/$REGISTRY_NAME/metamorph-demo:latest|g" deploy/digitalocean.yml

# Deploy to Kubernetes
print_status "Deploying Metamorph to Kubernetes..."
kubectl apply -f deploy/digitalocean.yml
print_success "✅ Metamorph deployed to Kubernetes"

# Wait for deployment to be ready
print_status "Waiting for deployment to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/metamorph-node -n metamorph-bsv
print_success "✅ Deployment is ready"

# Get service information
print_status "Getting service information..."
kubectl get services -n metamorph-bsv

# Get LoadBalancer IP
print_status "Waiting for LoadBalancer IP..."
EXTERNAL_IP=""
while [ -z $EXTERNAL_IP ]; do
    echo "Waiting for external IP..."
    EXTERNAL_IP=$(kubectl get svc metamorph-loadbalancer -n metamorph-bsv --template="{{range .status.loadBalancer.ingress}}{{.ip}}{{end}}")
    [ -z "$EXTERNAL_IP" ] && sleep 10
done

echo
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                 DEPLOYMENT SUCCESSFUL!                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo
print_success "🎉 Metamorph Bitcoin SV Node deployed successfully!"
echo
print_success "📊 Cluster: $CLUSTER_NAME"
print_success "🌐 External IP: $EXTERNAL_IP"
print_success "🔗 API Gateway: http://$EXTERNAL_IP/"
print_success "📈 Telemetry: http://$EXTERNAL_IP:9090/metrics"
print_success "🔍 Health Check: http://$EXTERNAL_IP:9090/health"
echo
print_status "🚀 Your Metamorph node is now running in production on DigitalOcean!"
print_status "📋 Use 'kubectl get pods -n metamorph-bsv' to check pod status"
print_status "📊 Use 'kubectl logs -f deployment/metamorph-node -n metamorph-bsv' to view logs"
echo
