# Bitcoin SV Mining Operations Guide

This comprehensive guide covers the operation, monitoring, and optimization of your production-grade Metamorph Bitcoin SV mining node.

## 🎯 Production Mining Node Overview

Your Metamorph Bitcoin SV node is now a **fully operational mining node** capable of:

- **Real Bitcoin SV Mining**: Live block template creation and proof-of-work mining
- **Mining Pool Integration**: Stratum protocol support for pool mining operations
- **Solo Mining**: Independent block mining with direct Bitcoin SV network connectivity
- **Auto-Scaling**: Dynamic scaling based on mining load and performance metrics
- **Production Monitoring**: Comprehensive metrics, logging, and alerting

## 🚀 Quick Start - Production Deployment

### Deploy to DigitalOcean
```bash
# Set your DigitalOcean API token
export DIGITAL_OCEAN_TOKEN=your_production_token

# Deploy production mining node
./deploy-mining-production.sh
```

### Expected Output
```
🎉 Bitcoin SV Mining Node Deployment Successful!
🌐 External IP: 178.128.134.166
⛏️  Mining Configuration: Bitcoin SV Testnet (LIVE mode)
👷 Workers: Auto-scaling 2-10 replicas
```

## ⛏️ Mining Configuration

### Solo Mining (Default)
```bash
# Current configuration - solo mining on testnet
BITCOIN_LIVE_MODE=true
BITCOIN_NETWORK=testnet
BITCOIN_MAX_WORKERS=8
```

### Pool Mining Setup
```bash
# Configure mining pool credentials
kubectl patch secret metamorph-mining-secrets -n metamorph-mining \
  -p='{"stringData":{"BITCOIN_POOL_URL":"stratum+tcp://pool.example.com:4444"}}'

kubectl patch secret metamorph-mining-secrets -n metamorph-mining \
  -p='{"stringData":{"BITCOIN_POOL_USERNAME":"your_username"}}'

kubectl patch secret metamorph-mining-secrets -n metamorph-mining \
  -p='{"stringData":{"BITCOIN_POOL_PASSWORD":"your_password"}}'

# Restart mining pods to apply new configuration
kubectl rollout restart deployment/metamorph-mining-node -n metamorph-mining
```

### Mainnet Mining (Production)
```bash
# Switch to Bitcoin SV mainnet for production mining
kubectl patch configmap metamorph-mining-config -n metamorph-mining \
  -p='{"data":{"BITCOIN_NETWORK":"mainnet"}}'

# Update mining address for mainnet rewards
kubectl patch secret metamorph-mining-secrets -n metamorph-mining \
  -p='{"stringData":{"BITCOIN_MINING_ADDRESS":"your_mainnet_address"}}'

# Restart to apply mainnet configuration
kubectl rollout restart deployment/metamorph-mining-node -n metamorph-mining
```

## 📊 Monitoring and Metrics

### Real-Time Mining Status
```bash
# Check mining pod status
kubectl get pods -n metamorph-mining

# View live mining logs
kubectl logs -f deployment/metamorph-mining-node -n metamorph-mining

# Monitor auto-scaling
kubectl describe hpa metamorph-mining-hpa -n metamorph-mining
```

### Mining Performance Metrics
```bash
# Access metrics endpoint
curl http://YOUR_EXTERNAL_IP:9090/metrics

# Key mining metrics to monitor:
# - mining_hash_rate_total
# - mining_blocks_found_total
# - mining_shares_submitted_total
# - mining_workers_active
# - mining_pool_connected
```

### Mining Dashboard URLs
- **Portal API**: `http://YOUR_EXTERNAL_IP/`
- **Metrics**: `http://YOUR_EXTERNAL_IP:9090/metrics`
- **Health Check**: `http://YOUR_EXTERNAL_IP/health`

## 🔧 Operations Management

### Scaling Mining Operations
```bash
# Scale mining workers manually
kubectl patch configmap metamorph-mining-config -n metamorph-mining \
  -p='{"data":{"BITCOIN_MAX_WORKERS":"16"}}'

# Scale Kubernetes replicas
kubectl scale deployment metamorph-mining-node --replicas=5 -n metamorph-mining

# Update auto-scaling limits
kubectl patch hpa metamorph-mining-hpa -n metamorph-mining \
  -p='{"spec":{"maxReplicas":20}}'
```

### Resource Optimization
```bash
# Update resource limits for high-performance mining
kubectl patch deployment metamorph-mining-node -n metamorph-mining \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"mining-node","resources":{"limits":{"cpu":"8000m","memory":"16Gi"}}}]}}}}'

# Monitor resource usage
kubectl top pods -n metamorph-mining
kubectl top nodes
```

### Configuration Updates
```bash
# Update mining configuration
kubectl edit configmap metamorph-mining-config -n metamorph-mining

# Update mining secrets (pool credentials, addresses)
kubectl edit secret metamorph-mining-secrets -n metamorph-mining

# Apply configuration changes
kubectl rollout restart deployment/metamorph-mining-node -n metamorph-mining
```

## 📈 Performance Optimization

### Mining Performance Tuning
```yaml
# Optimal configuration for high-performance mining
BITCOIN_MAX_WORKERS: "16"        # Match CPU cores
BITCOIN_MIN_WORKERS: "4"         # Minimum active workers
BITCOIN_MAX_PEERS: "32"          # Increase peer connections
GO_MAX_PROCS: "16"               # Match container CPU limit
HASH_RATE_TARGET: "10000000"     # Target hash rate (10 MH/s)
```

### Hardware Recommendations
- **CPU**: 8+ cores (Intel Xeon or AMD EPYC)
- **Memory**: 16+ GB RAM
- **Storage**: 200+ GB SSD (blockchain data)
- **Network**: 1+ Gbps bandwidth
- **Nodes**: 3+ Kubernetes nodes for high availability

### Auto-Scaling Configuration
```yaml
# HPA configuration for mining workload
minReplicas: 2
maxReplicas: 20
targetCPUUtilizationPercentage: 70
targetMemoryUtilizationPercentage: 80
```

## 🛡️ Security and Maintenance

### Security Best Practices
```bash
# Enable TLS for API endpoints
kubectl patch configmap metamorph-mining-config -n metamorph-mining \
  -p='{"data":{"ENABLE_TLS":"true"}}'

# Rotate mining credentials regularly
kubectl create secret generic metamorph-mining-secrets-new \
  --from-literal=BITCOIN_POOL_PASSWORD=new_secure_password \
  -n metamorph-mining

# Update network policies for security
kubectl apply -f k8s/network-policies.yaml
```

### Backup and Recovery
```bash
# Backup mining configuration
kubectl get configmap metamorph-mining-config -n metamorph-mining -o yaml > mining-config-backup.yaml
kubectl get secret metamorph-mining-secrets -n metamorph-mining -o yaml > mining-secrets-backup.yaml

# Backup blockchain data (if persistent storage is used)
kubectl exec -it deployment/metamorph-mining-node -n metamorph-mining -- tar -czf /tmp/blockchain-backup.tar.gz /data/blockchain
```

### Maintenance Windows
```bash
# Schedule maintenance during low mining activity
kubectl patch deployment metamorph-mining-node -n metamorph-mining \
  -p='{"spec":{"replicas":1}}'

# Perform updates
kubectl set image deployment/metamorph-mining-node mining-node=new-image:latest -n metamorph-mining

# Restore full capacity
kubectl patch deployment metamorph-mining-node -n metamorph-mining \
  -p='{"spec":{"replicas":5}}'
```

## 📊 Mining Analytics and Reporting

### Daily Mining Report
```bash
# Generate daily mining statistics
kubectl exec -it deployment/metamorph-mining-node -n metamorph-mining -- \
  curl -s http://localhost:9090/metrics | grep mining_

# Key metrics to track:
# - Total hash rate over 24 hours
# - Blocks found and rewards earned
# - Pool shares submitted and accepted
# - Mining efficiency and uptime
```

### Performance Benchmarking
```bash
# Benchmark mining performance
kubectl exec -it deployment/metamorph-mining-node -n metamorph-mining -- \
  go run ./cmd/mining-demo/main.go --benchmark --duration=300s

# Compare performance across different configurations
# Test different worker counts, pool vs solo mining, etc.
```

## 🚨 Troubleshooting

### Common Issues and Solutions

#### Mining Not Starting
```bash
# Check pod status and logs
kubectl describe pod -l app=metamorph-mining -n metamorph-mining
kubectl logs -f deployment/metamorph-mining-node -n metamorph-mining

# Common causes:
# - Invalid mining address
# - Network connectivity issues
# - Resource constraints
```

#### Low Hash Rate
```bash
# Check resource allocation
kubectl top pods -n metamorph-mining

# Increase worker count
kubectl patch configmap metamorph-mining-config -n metamorph-mining \
  -p='{"data":{"BITCOIN_MAX_WORKERS":"32"}}'

# Scale up resources
kubectl patch deployment metamorph-mining-node -n metamorph-mining \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"mining-node","resources":{"limits":{"cpu":"16000m"}}}]}}}}'
```

#### Pool Connection Issues
```bash
# Verify pool credentials
kubectl get secret metamorph-mining-secrets -n metamorph-mining -o yaml

# Test pool connectivity
kubectl exec -it deployment/metamorph-mining-node -n metamorph-mining -- \
  telnet pool.example.com 4444

# Check pool-specific logs
kubectl logs -f deployment/metamorph-mining-node -n metamorph-mining | grep -i pool
```

#### Network Connectivity Problems
```bash
# Check Bitcoin SV peer connections
kubectl exec -it deployment/metamorph-mining-node -n metamorph-mining -- \
  netstat -an | grep :8333

# Verify DNS resolution
kubectl exec -it deployment/metamorph-mining-node -n metamorph-mining -- \
  nslookup seed.bitcoinsv.io

# Check firewall rules
kubectl get networkpolicy -n metamorph-mining
```

## 💰 Mining Economics and ROI

### Revenue Calculation
```bash
# Calculate potential mining revenue
# Factors to consider:
# - Current Bitcoin SV price
# - Network hash rate and difficulty
# - Your node's hash rate contribution
# - Pool fees (if applicable)
# - Electricity and hosting costs

# Example calculation for testnet (educational purposes):
# Hash Rate: 10 MH/s
# Network Difficulty: Current testnet difficulty
# Block Reward: 62.5 BSV
# Expected blocks per day: (Your hash rate / Network hash rate) * 144 blocks
```

### Cost Optimization
```bash
# Monitor resource costs
kubectl get nodes -o wide
doctl kubernetes cluster get metamorph-mining-production

# Optimize for cost-efficiency:
# - Use spot instances where available
# - Scale down during low-profitability periods
# - Optimize resource allocation based on mining performance
```

## 🌍 Multi-Region Deployment

### Global Mining Operations
```bash
# Deploy to multiple regions for redundancy
./deploy-mining-production.sh --region=nyc1
./deploy-mining-production.sh --region=sfo3
./deploy-mining-production.sh --region=ams3

# Load balance mining operations across regions
# Implement failover for high availability
```

## 📚 Advanced Features

### Custom Mining Strategies
- **Difficulty-based scaling**: Scale workers based on network difficulty
- **Profitability switching**: Switch between solo and pool mining
- **Multi-pool support**: Connect to multiple pools simultaneously
- **Smart routing**: Route mining power to most profitable pools

### Integration with Bitcoin SV Ecosystem
- **Merchant APIs**: Integrate with Bitcoin SV merchant services
- **SPV Wallets**: Provide SPV wallet functionality
- **Payment Channels**: Implement Bitcoin SV payment channels
- **Smart Contracts**: Execute Bitcoin SV smart contracts

## 🎯 Success Metrics

### Key Performance Indicators (KPIs)
- **Hash Rate**: Target >10 MH/s sustained
- **Uptime**: >99.9% mining availability
- **Efficiency**: <1% stale/rejected shares
- **Profitability**: Positive ROI after costs
- **Network Contribution**: Meaningful contribution to Bitcoin SV network security

---

## 🚀 Ready for Production Mining!

Your Metamorph Bitcoin SV mining node is now **production-ready** and capable of:

✅ **Real Bitcoin SV Mining** on mainnet/testnet  
✅ **Mining Pool Integration** with Stratum protocol  
✅ **Auto-Scaling** based on mining performance  
✅ **Production Monitoring** with comprehensive metrics  
✅ **High Availability** with multi-replica deployment  

**Next Steps**: Deploy to production, configure mining pools, and start earning Bitcoin SV rewards!

**Repository**: git@github.com:codenlighten/bitcoin-sv-optimized-node.git  
**Status**: 🎯 **PRODUCTION MINING READY**
