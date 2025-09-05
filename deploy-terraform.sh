#!/bin/bash

# Deploy Metamorph using Terraform with proper permissions
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║            METAMORPH TERRAFORM DEPLOYMENT                   ║"
echo "║          Alternative Production Deployment                   ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

# Load environment variables
if [ -f ".env" ]; then
    export $(cat .env | xargs)
    print_success "✅ Environment variables loaded"
else
    print_warning "⚠️  .env file not found, please ensure DIGITAL_OCEAN_TOKEN is set"
fi

# Check if terraform is installed
if ! command -v terraform &> /dev/null; then
    print_status "Installing Terraform..."
    
    # Install Terraform
    wget -O- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list
    sudo apt update && sudo apt install terraform
    
    print_success "✅ Terraform installed"
fi

# Navigate to terraform directory
cd terraform/

# Create terraform.tfvars if it doesn't exist
if [ ! -f "terraform.tfvars" ]; then
    print_status "Creating terraform.tfvars..."
    cat > terraform.tfvars << EOF
do_token = "$DIGITAL_OCEAN_TOKEN"
cluster_name = "metamorph-bsv-terraform"
region = "nyc1"
node_size = "s-4vcpu-8gb"
min_nodes = 3
max_nodes = 10
EOF
    print_success "✅ terraform.tfvars created"
fi

# Initialize Terraform
print_status "Initializing Terraform..."
terraform init

# Plan the deployment
print_status "Planning Terraform deployment..."
terraform plan

# Apply the deployment
print_status "Applying Terraform deployment..."
terraform apply -auto-approve

# Get outputs
print_status "Getting deployment outputs..."
CLUSTER_ID=$(terraform output -raw cluster_id)
REGISTRY_ENDPOINT=$(terraform output -raw registry_endpoint)
LB_IP=$(terraform output -raw loadbalancer_ip)

print_success "🎉 Terraform deployment completed!"
echo
print_success "📊 Cluster ID: $CLUSTER_ID"
print_success "🐳 Registry: $REGISTRY_ENDPOINT"
print_success "🌐 Load Balancer IP: $LB_IP"
echo

# Configure kubectl with the new cluster
print_status "Configuring kubectl..."
doctl kubernetes cluster kubeconfig save $CLUSTER_ID

# Deploy Metamorph services
print_status "Deploying Metamorph services..."
cd ..
kubectl apply -f deploy/digitalocean.yml

print_success "🚀 Metamorph Bitcoin SV Node deployed via Terraform!"
print_status "📋 Use 'kubectl get pods -n metamorph-bsv' to check status"
