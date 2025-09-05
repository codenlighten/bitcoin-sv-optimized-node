# Metamorph Bitcoin SV Node - Cloud Deployment Guide

## 🚀 DigitalOcean Production Deployment

This guide provides comprehensive instructions for deploying your Metamorph Bitcoin SV node to DigitalOcean's production infrastructure.

### Prerequisites

1. **DigitalOcean Account** with API token
2. **Docker** installed locally
3. **Git** repository access
4. **Domain name** (optional, for custom domains)

### Quick Deployment (Automated)

The fastest way to deploy Metamorph to production:

```bash
# 1. Ensure your DIGITAL_OCEAN_TOKEN is in .env
echo "DIGITAL_OCEAN_TOKEN=your_token_here" > .env

# 2. Run the automated deployment script
./deploy-to-digitalocean.sh
```

This script will:
- ✅ Install required tools (doctl, kubectl)
- ✅ Create Kubernetes cluster with autoscaling
- ✅ Set up container registry
- ✅ Build and push Docker images
- ✅ Deploy all Metamorph services
- ✅ Configure load balancer and networking
- ✅ Provide access URLs and monitoring endpoints

### Manual Deployment Steps

For more control over the deployment process:

#### 1. Infrastructure Setup with Terraform

```bash
cd terraform/

# Copy and edit variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your DigitalOcean token

# Initialize and apply Terraform
terraform init
terraform plan
terraform apply
```

#### 2. Kubernetes Deployment

```bash
# Configure kubectl with your cluster
doctl kubernetes cluster kubeconfig save metamorph-bsv-cluster

# Deploy Metamorph services
kubectl apply -f deploy/digitalocean.yml

# Check deployment status
kubectl get pods -n metamorph-bsv
kubectl get services -n metamorph-bsv
```

### Infrastructure Components

Your Metamorph deployment includes:

#### **Kubernetes Cluster**
- **Node Pool**: 3-10 nodes with autoscaling
- **Instance Type**: 4 vCPU, 8GB RAM (configurable)
- **Region**: NYC1 (configurable)
- **Networking**: Private VPC with security groups

#### **Services Deployed**
- **Metamorph Node**: Core Bitcoin SV node (3 replicas)
- **Load Balancer**: External traffic distribution
- **Container Registry**: Private Docker image storage
- **Persistent Storage**: 100GB block storage
- **Redis Database**: Production UTXO caching
- **Monitoring**: Health checks and metrics

#### **Networking & Security**
- **Load Balancer**: HTTP/HTTPS termination
- **Firewall Rules**: Secure port access
- **Network Policies**: Pod-to-pod communication
- **TLS Certificates**: Automatic SSL/TLS

### Service Endpoints

Once deployed, your Metamorph node will be accessible at:

#### **Public APIs**
- **API Gateway**: `http://YOUR_LB_IP/` (Bitcoin SV node API)
- **Health Checks**: `http://YOUR_LB_IP:9090/health`
- **Metrics**: `http://YOUR_LB_IP:9090/metrics`
- **P2P Network**: `YOUR_LB_IP:8333` (Bitcoin P2P)

#### **ARC-Compatible Endpoints**
- **Transaction Submit**: `POST http://YOUR_LB_IP/v1/tx`
- **Transaction Status**: `GET http://YOUR_LB_IP/v1/tx/{txid}`
- **UTXO Lookup**: `POST http://YOUR_LB_IP/api/v1/utxo/lookup`

### Monitoring & Observability

#### **Health Monitoring**
```bash
# Check pod health
kubectl get pods -n metamorph-bsv

# View logs
kubectl logs -f deployment/metamorph-node -n metamorph-bsv

# Check resource usage
kubectl top pods -n metamorph-bsv
```

#### **Metrics Collection**
- **Prometheus-style metrics**: Available at `/metrics` endpoint
- **Custom Bitcoin SV metrics**: Transaction validation, UTXO operations
- **Infrastructure metrics**: CPU, memory, network, storage

#### **Alerting**
Configure alerts for:
- Pod failures or restarts
- High CPU/memory usage
- Transaction validation errors
- P2P connection issues
- Storage capacity warnings

### Scaling Configuration

#### **Horizontal Pod Autoscaler**
Automatic scaling based on:
- **CPU Usage**: Scale at 70% utilization
- **Memory Usage**: Scale at 80% utilization
- **Custom Metrics**: Transaction queue depth

#### **Cluster Autoscaler**
Node pool scaling:
- **Minimum Nodes**: 3
- **Maximum Nodes**: 10
- **Scale-up**: When pods can't be scheduled
- **Scale-down**: When nodes are underutilized

### Cost Optimization

#### **Resource Limits**
- **CPU Requests**: 500m per pod
- **CPU Limits**: 2000m per pod
- **Memory Requests**: 512Mi per pod
- **Memory Limits**: 2Gi per pod

#### **Storage Optimization**
- **Block Storage**: 100GB SSD (expandable)
- **Container Registry**: Professional tier
- **Database**: Single node Redis (upgradeable)

#### **Estimated Monthly Costs**
- **Kubernetes Cluster**: ~$120/month (3 nodes)
- **Load Balancer**: ~$12/month
- **Container Registry**: ~$20/month
- **Database**: ~$15/month
- **Storage**: ~$10/month
- **Total**: ~$177/month for production setup

### Security Best Practices

#### **Network Security**
- Private VPC with controlled ingress
- Network policies for pod communication
- Firewall rules for specific ports only
- TLS encryption for all external traffic

#### **Container Security**
- Non-root container execution
- Read-only root filesystem
- Security context constraints
- Regular image vulnerability scanning

#### **Access Control**
- RBAC for Kubernetes access
- Service account permissions
- API key authentication for endpoints
- Audit logging enabled

### Backup & Disaster Recovery

#### **Data Backup**
```bash
# Backup UTXO data
kubectl exec -it deployment/metamorph-node -n metamorph-bsv -- /backup-script.sh

# Backup to DigitalOcean Spaces
# (Automated via scheduled jobs)
```

#### **Disaster Recovery**
- **RTO**: Recovery Time Objective < 15 minutes
- **RPO**: Recovery Point Objective < 5 minutes
- **Multi-region**: Deploy to multiple regions for HA
- **Database Replication**: Redis clustering for production

### Troubleshooting

#### **Common Issues**

**Pods not starting:**
```bash
kubectl describe pod <pod-name> -n metamorph-bsv
kubectl logs <pod-name> -n metamorph-bsv
```

**Load balancer not accessible:**
```bash
kubectl get svc metamorph-loadbalancer -n metamorph-bsv
# Check DigitalOcean firewall rules
```

**High resource usage:**
```bash
kubectl top pods -n metamorph-bsv
# Check HPA scaling
kubectl get hpa -n metamorph-bsv
```

#### **Support Contacts**
- **DigitalOcean Support**: For infrastructure issues
- **Kubernetes Documentation**: For cluster management
- **Metamorph Repository**: For application-specific issues

### Next Steps

After successful deployment:

1. **Configure DNS** - Point your domain to the load balancer IP
2. **Set up SSL/TLS** - Configure certificates for HTTPS
3. **Enable Monitoring** - Set up Grafana dashboards
4. **Configure Alerts** - Set up PagerDuty or similar
5. **Performance Tuning** - Optimize based on actual usage
6. **Security Hardening** - Implement additional security measures

### Production Checklist

- [ ] Infrastructure deployed via Terraform
- [ ] Kubernetes cluster configured and accessible
- [ ] All Metamorph services running (3+ replicas)
- [ ] Load balancer configured with health checks
- [ ] DNS configured (if using custom domain)
- [ ] SSL/TLS certificates configured
- [ ] Monitoring and alerting set up
- [ ] Backup procedures tested
- [ ] Security policies implemented
- [ ] Performance benchmarks established
- [ ] Documentation updated with production details

---

**🎉 Congratulations! Your Metamorph Bitcoin SV node is now running in production on DigitalOcean!**

For additional support or advanced configurations, refer to the main project documentation or create an issue in the repository.
