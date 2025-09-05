# Bitcoin SV Live Network Connectivity Guide

This guide explains how to connect your Metamorph Bitcoin SV node to the live Bitcoin SV network for real transaction processing and blockchain synchronization.

## 🌐 Live Network Features

Your Metamorph node now supports **real Bitcoin SV network connectivity** with the following capabilities:

### ✅ Live Network Connectivity
- **Real P2P Networking**: Connect to live Bitcoin SV peers via DNS seed discovery
- **Transaction Relay**: Receive and validate real Bitcoin SV transactions
- **Block Synchronization**: Download and validate real Bitcoin SV blocks
- **Network Health Monitoring**: Automatic peer management and connection health

### ✅ Dual Mode Operation
- **Live Mode**: Connect to real Bitcoin SV mainnet/testnet
- **Demo Mode**: Use simulated networking for development and testing
- **Seamless Switching**: Toggle between modes via environment variables

## 🚀 Quick Start - Enable Live Network

### Step 1: Configure Live Mode
```bash
# Copy the live network configuration
cp .env.live .env

# Edit configuration as needed
nano .env
```

### Step 2: Set Network Parameters
```bash
# For Bitcoin SV Testnet (recommended for testing)
export BITCOIN_LIVE_MODE=true
export BITCOIN_NETWORK=testnet

# For Bitcoin SV Mainnet (production)
export BITCOIN_LIVE_MODE=true
export BITCOIN_NETWORK=mainnet
```

### Step 3: Start Live Node
```bash
# Run with live network connectivity
go run ./cmd/demo/main.go

# Or build and run
go build -o metamorph ./cmd/demo/main.go
./metamorph
```

## 📊 Live Network Status

When live mode is enabled, you'll see:

```
🌐 Sentinel P2P Service: Starting LIVE Bitcoin SV testnet networking...
🌐 NetworkManager: Connecting to Bitcoin SV testnet network...
🔍 NetworkManager: Discovered 12 potential peers
✅ NetworkManager: Connected to peer 95.217.161.135:18333
✅ NetworkManager: Connected to peer 144.76.118.2:18333
🌐 NetworkManager: Connected to Bitcoin SV testnet network
🌐 Sentinel: Bitcoin SV P2P service started on testnet (LIVE mode)
```

## 🔧 Configuration Options

### Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `BITCOIN_LIVE_MODE` | Enable live network connectivity | `false` | `true` |
| `BITCOIN_NETWORK` | Network selection | `testnet` | `mainnet`, `testnet` |
| `BITCOIN_MAX_PEERS` | Maximum peer connections | `8` | `16` |
| `BITCOIN_MIN_PEERS` | Minimum peer connections | `3` | `5` |

### Network Selection

#### Testnet (Recommended for Development)
```bash
BITCOIN_NETWORK=testnet
```
- **Purpose**: Safe testing environment
- **Port**: 18333
- **DNS Seeds**: testnet-seed.bitcoinsv.io, testnet-seed.cascharia.com
- **Block Explorer**: https://test.whatsonchain.com/

#### Mainnet (Production)
```bash
BITCOIN_NETWORK=mainnet
```
- **Purpose**: Live Bitcoin SV network
- **Port**: 8333
- **DNS Seeds**: seed.bitcoinsv.io, seed.cascharia.com, seed.satoshisvision.network
- **Block Explorer**: https://whatsonchain.com/

## 📡 Live Network Events

When connected to the live network, your node will publish these events:

### P2P Events
- `p2p.peer_connected.v1` - New peer connections
- `p2p.peer_disconnected.v1` - Peer disconnections
- `p2p.raw_message.v1` - Raw Bitcoin SV protocol messages
- `p2p.network_health.v1` - Network connectivity metrics

### Transaction Events
- `p2p.raw_tx.v1` - Real Bitcoin SV transactions from network
- `tx.validated.v1` - Validated transactions ready for processing
- `tx.validation_failed.v1` - Invalid transactions rejected

### Block Events
- `p2p.raw_block.v1` - Real Bitcoin SV blocks from network
- `block.validated.v1` - Validated blocks ready for processing

## 🔍 Monitoring Live Network

### Real-Time Logs
```bash
# Monitor live network activity
tail -f metamorph.log | grep "NetworkManager\|LIVE"

# Monitor peer connections
tail -f metamorph.log | grep "peer_connected\|peer_disconnected"

# Monitor transaction flow
tail -f metamorph.log | grep "raw_tx\|validated"
```

### Network Health Metrics
```bash
# Check network status via API (if Portal service is running)
curl http://localhost:8080/health/network

# View peer statistics
curl http://localhost:8080/api/v1/peers
```

## 🚀 Production Deployment

### Deploy to DigitalOcean with Live Network
```bash
# Set live mode in deployment configuration
echo "BITCOIN_LIVE_MODE=true" >> .env

# Deploy to Kubernetes with live network enabled
./deploy-to-digitalocean.sh
```

### Kubernetes Configuration
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: metamorph-config
data:
  BITCOIN_LIVE_MODE: "true"
  BITCOIN_NETWORK: "testnet"
  BITCOIN_MAX_PEERS: "8"
```

## ⚠️ Important Considerations

### Security
- **Testnet First**: Always test on testnet before mainnet
- **Firewall Rules**: Ensure ports 8333 (mainnet) or 18333 (testnet) are accessible
- **Resource Monitoring**: Live network increases CPU and bandwidth usage

### Performance
- **Bandwidth**: Live network requires sustained internet connectivity
- **Storage**: Blockchain data will accumulate over time
- **Memory**: Peer connections and message buffers increase memory usage

### Compliance
- **Legal**: Ensure compliance with local regulations for Bitcoin SV node operation
- **Network**: Follow Bitcoin SV network protocol standards and best practices

## 🛠️ Troubleshooting

### Common Issues

#### No Peer Connections
```bash
# Check DNS resolution
nslookup seed.bitcoinsv.io

# Check port connectivity
telnet seed.bitcoinsv.io 8333

# Verify firewall settings
sudo ufw status
```

#### High Resource Usage
```bash
# Monitor resource usage
htop

# Check peer count
grep "Managing.*peers" metamorph.log

# Reduce max peers if needed
export BITCOIN_MAX_PEERS=4
```

#### Connection Timeouts
```bash
# Increase connection timeout
export BITCOIN_CONNECTION_TIMEOUT=30s

# Check network latency
ping seed.bitcoinsv.io
```

## 📈 Next Steps

With live network connectivity enabled, you can:

1. **Real Transaction Processing**: Process actual Bitcoin SV transactions
2. **Blockchain Synchronization**: Download and validate the Bitcoin SV blockchain
3. **Mining Integration**: Connect to mining pools for block production
4. **Wallet Services**: Implement SPV and wallet functionality
5. **Ecosystem Integration**: Connect to Bitcoin SV services and applications

## 🎯 Advanced Configuration

### Custom Peer Discovery
```go
// Add custom peer addresses
customPeers := []string{
    "95.217.161.135:8333",
    "144.76.118.2:8333",
}
```

### Message Filtering
```go
// Filter specific message types
messageTypes := []string{"tx", "block", "inv"}
```

### Performance Tuning
```bash
# Optimize for high throughput
export BITCOIN_MAX_PEERS=16
export BITCOIN_CONNECTION_TIMEOUT=5s
export ENABLE_MESSAGE_BATCHING=true
```

---

**Status**: ✅ Live Bitcoin SV network connectivity ready
**Repository**: git@github.com:codenlighten/bitcoin-sv-optimized-node.git
**Support**: Real Bitcoin SV P2P networking with production-grade peer management
