# Terraform configuration for Metamorph Bitcoin SV Node on DigitalOcean
# Production-grade infrastructure with monitoring and scaling

terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
}

# Configure the DigitalOcean Provider
provider "digitalocean" {
  token = var.do_token
}

# Variables
variable "do_token" {
  description = "DigitalOcean API Token"
  type        = string
  sensitive   = true
}

variable "cluster_name" {
  description = "Name of the Kubernetes cluster"
  type        = string
  default     = "metamorph-bsv-cluster"
}

variable "region" {
  description = "DigitalOcean region"
  type        = string
  default     = "nyc1"
}

variable "node_size" {
  description = "Size of the worker nodes"
  type        = string
  default     = "s-4vcpu-8gb"
}

variable "min_nodes" {
  description = "Minimum number of nodes"
  type        = number
  default     = 3
}

variable "max_nodes" {
  description = "Maximum number of nodes"
  type        = number
  default     = 10
}

# Create a VPC
resource "digitalocean_vpc" "metamorph_vpc" {
  name     = "metamorph-vpc"
  region   = var.region
  ip_range = "10.10.0.0/16"
}

# Create Kubernetes cluster
resource "digitalocean_kubernetes_cluster" "metamorph_cluster" {
  name     = var.cluster_name
  region   = var.region
  version  = "1.28.2-do.0"
  vpc_uuid = digitalocean_vpc.metamorph_vpc.id

  node_pool {
    name       = "metamorph-pool"
    size       = var.node_size
    node_count = var.min_nodes
    auto_scale = true
    min_nodes  = var.min_nodes
    max_nodes  = var.max_nodes
    
    labels = {
      service = "metamorph"
      environment = "production"
    }
    
    taint {
      key    = "workloadKind"
      value  = "database"
      effect = "NoSchedule"
    }
  }

  tags = ["metamorph", "bitcoin-sv", "production"]
}

# Create Container Registry
resource "digitalocean_container_registry" "metamorph_registry" {
  name                   = "metamorph-registry"
  subscription_tier_slug = "professional"
  region                 = "nyc3"
}

# Create Load Balancer
resource "digitalocean_loadbalancer" "metamorph_lb" {
  name   = "metamorph-bsv-lb"
  region = var.region
  vpc_uuid = digitalocean_vpc.metamorph_vpc.id

  forwarding_rule {
    entry_protocol  = "http"
    entry_port      = 80
    target_protocol = "http"
    target_port     = 8080
  }

  forwarding_rule {
    entry_protocol  = "tcp"
    entry_port      = 8333
    target_protocol = "tcp"
    target_port     = 8333
  }

  forwarding_rule {
    entry_protocol  = "http"
    entry_port      = 9090
    target_protocol = "http"
    target_port     = 9090
  }

  healthcheck {
    protocol               = "http"
    port                   = 9090
    path                   = "/health"
    check_interval_seconds = 10
    response_timeout_seconds = 5
    unhealthy_threshold    = 3
    healthy_threshold      = 2
  }

  droplet_tag = "metamorph-node"
}

# Create Firewall
resource "digitalocean_firewall" "metamorph_firewall" {
  name = "metamorph-firewall"

  droplet_ids = []

  # Allow HTTP traffic
  inbound_rule {
    protocol         = "tcp"
    port_range       = "80"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  # Allow HTTPS traffic
  inbound_rule {
    protocol         = "tcp"
    port_range       = "443"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  # Allow Bitcoin P2P traffic
  inbound_rule {
    protocol         = "tcp"
    port_range       = "8333"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  # Allow monitoring traffic
  inbound_rule {
    protocol         = "tcp"
    port_range       = "9090"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  # Allow SSH
  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  # Allow all outbound traffic
  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}

# Create Database for production UTXO storage
resource "digitalocean_database_cluster" "metamorph_db" {
  name       = "metamorph-db"
  engine     = "redis"
  version    = "7"
  size       = "db-s-1vcpu-1gb"
  region     = var.region
  node_count = 1

  tags = ["metamorph", "production"]
}

# Create Spaces bucket for backups
resource "digitalocean_spaces_bucket" "metamorph_backups" {
  name   = "metamorph-backups"
  region = "nyc3"
  
  versioning {
    enabled = true
  }
}

# Outputs
output "cluster_id" {
  description = "ID of the Kubernetes cluster"
  value       = digitalocean_kubernetes_cluster.metamorph_cluster.id
}

output "cluster_endpoint" {
  description = "Endpoint of the Kubernetes cluster"
  value       = digitalocean_kubernetes_cluster.metamorph_cluster.endpoint
}

output "registry_endpoint" {
  description = "Container registry endpoint"
  value       = digitalocean_container_registry.metamorph_registry.endpoint
}

output "loadbalancer_ip" {
  description = "Load balancer IP address"
  value       = digitalocean_loadbalancer.metamorph_lb.ip
}

output "database_uri" {
  description = "Database connection URI"
  value       = digitalocean_database_cluster.metamorph_db.uri
  sensitive   = true
}

output "spaces_bucket_domain" {
  description = "Spaces bucket domain"
  value       = digitalocean_spaces_bucket.metamorph_backups.bucket_domain_name
}
