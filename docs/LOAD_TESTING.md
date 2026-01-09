# Production Load Testing Guide

This guide explains how to get production-realistic performance numbers from Osprey.

## Why Local Tests Are Misleading

| Factor | Local | Production |
|--------|-------|------------|
| Database | SQLite (in-process) | PostgreSQL (network) |
| Cache | In-memory (instant) | Redis (network) |
| Message Bus | Go channels (instant) | NATS (network) |
| Network | Localhost (0ms) | Real latency (1-50ms) |
| Disk I/O | SSD, no contention | Shared, IOPS limits |
| CPU | Full machine | Container limits |

**Local TPS: ~4,000/s** vs **Production TPS: ~1,000-2,000/s** (typical)

---

## Quick Start

### Option 1: Test Against Docker Stack (Recommended)

This gives you production-like infrastructure locally:

```bash
# Start the full stack (PostgreSQL, Redis, NATS, Osprey)
docker-compose up -d

# Wait for healthy
curl http://localhost:8080/health

# Seed rules
./scripts/seed-rules.sh

# Run load test
k6 run k6/production-load-test.js

# Stop stack
docker-compose down
```

### Option 2: One-Command Test

```bash
./scripts/load-test.sh docker
```

### Option 3: Test Remote Server

```bash
k6 run -e BASE_URL=https://osprey.example.com k6/production-load-test.js
```

---

## Test Phases

The production load test runs 4 phases to simulate real traffic patterns:

```
VUs
 ^
 |         ┌──────────────┐
 |        /│              │\
 |       / │   Sustained  │ \    Spike
 |      /  │    (3 min)   │  \   ╱╲
 |     /   │              │   \ ╱  ╲
 |    /    │              │    ╳    ╲
 |   /     │              │   ╱ ╲    ╲
 |──/──────┴──────────────┴──╱───╲────╲───────
 0   2min       5min              6min    7.5min
     Ramp-up                Cool-down

Phase 1: Ramp-up (2 min)    - Gradual traffic increase
Phase 2: Sustained (3 min)  - Hold at max load
Phase 3: Spike (1 min)      - 2x traffic burst
Phase 4: Cool-down (1.5 min) - Gradual decrease
```

---

## Metrics & SLAs

### Thresholds (must pass)

| Metric | Threshold | Meaning |
|--------|-----------|---------|
| p50 latency | < 10ms | Half of requests very fast |
| p95 latency | < 50ms | Most requests fast |
| p99 latency | < 100ms | Tail latency acceptable |
| Error rate | < 0.1% | 99.9% availability |

### Custom Metrics

| Metric | Description |
|--------|-------------|
| `osprey_evaluation_duration` | End-to-end evaluation time |
| `osprey_alert_rate` | Percentage of transactions flagged |
| `osprey_errors` | Count of non-200 responses |

---

## Infrastructure Sizing

### Baseline (1,000 TPS)

```yaml
Osprey:
  replicas: 2
  cpu: 1 core
  memory: 512MB

PostgreSQL:
  cpu: 2 cores
  memory: 2GB
  storage: SSD, 1000 IOPS

Redis:
  cpu: 1 core
  memory: 1GB

NATS:
  cpu: 1 core
  memory: 512MB
```

### Scale (5,000 TPS)

```yaml
Osprey:
  replicas: 5
  cpu: 2 cores
  memory: 1GB

PostgreSQL:
  cpu: 4 cores
  memory: 8GB
  storage: SSD, 5000 IOPS
  replicas: 1 primary + 1 replica

Redis:
  cpu: 2 cores
  memory: 4GB
  mode: cluster (3 nodes)

NATS:
  cpu: 2 cores
  memory: 1GB
  mode: cluster (3 nodes)
```

---

## Running Load Tests

### Basic Test (100 VUs)

```bash
k6 run k6/production-load-test.js
```

### High Load Test (500 VUs)

```bash
k6 run -e MAX_VUS=500 k6/production-load-test.js
```

### Extended Duration Test

```bash
# Modify stages in production-load-test.js or use:
k6 run --duration 30m k6/quick-test.js
```

### Distributed Load Test

For very high loads, run from multiple machines:

```bash
# Machine 1
k6 run -e MAX_VUS=200 -e TENANT_ID=load-test-1 k6/production-load-test.js

# Machine 2
k6 run -e MAX_VUS=200 -e TENANT_ID=load-test-2 k6/production-load-test.js

# Machine 3
k6 run -e MAX_VUS=200 -e TENANT_ID=load-test-3 k6/production-load-test.js
```

---

## Monitoring During Tests

### Docker Stack Metrics

```bash
# Watch container resources
docker stats

# PostgreSQL connections
docker exec osprey-postgres psql -U osprey -c "SELECT count(*) FROM pg_stat_activity;"

# Redis stats
docker exec osprey-redis redis-cli info stats

# NATS stats
curl http://localhost:8222/varz | jq '.connections, .slow_consumers'
```

### Osprey Health

```bash
# Check mode and health
curl http://localhost:8080/health | jq

# Watch in loop
watch -n1 'curl -s http://localhost:8080/health | jq'
```

---

## Interpreting Results

### Good Results

```
📊 THROUGHPUT
   Total Requests:  50000
   Requests/sec:    1250.00
   Failed:          0.000%

⏱️  LATENCY
   Average:         5.23ms
   Median (p50):    3.50ms
   p95:             15.20ms
   p99:             45.30ms
   Max:             120.50ms
```

**What to look for:**
- Failed rate < 0.1%
- p95 < 50ms
- p99 < 100ms
- Consistent throughput during sustained phase

### Warning Signs

| Symptom | Possible Cause | Fix |
|---------|----------------|-----|
| p99 >> p95 | Connection pool exhaustion | Increase pool size |
| Increasing latency over time | Memory leak, GC pressure | Check heap usage |
| Errors during spike | Insufficient headroom | Scale horizontally |
| High Redis latency | Cache misses | Increase cache size |
| PostgreSQL timeouts | Slow queries, locks | Add indexes, check locks |

---

## Capacity Planning

Use this formula to estimate required capacity:

```
Required Instances = (Peak TPS × Safety Margin) / (TPS per Instance)

Example:
  Peak TPS: 5,000
  Safety Margin: 1.5 (50% headroom)
  TPS per Instance: 1,500 (from load test)

  Required = (5000 × 1.5) / 1500 = 5 instances
```

---

## Cloud-Specific Guidance

### AWS

```bash
# EKS with Fargate
eksctl create cluster --name osprey --fargate

# RDS PostgreSQL
aws rds create-db-instance \
  --db-instance-identifier osprey-db \
  --db-instance-class db.r6g.large \
  --engine postgres

# ElastiCache Redis
aws elasticache create-cache-cluster \
  --cache-cluster-id osprey-cache \
  --cache-node-type cache.r6g.large \
  --engine redis
```

### GCP

```bash
# GKE Autopilot
gcloud container clusters create-auto osprey

# Cloud SQL
gcloud sql instances create osprey-db \
  --database-version=POSTGRES_15 \
  --tier=db-custom-4-16384

# Memorystore Redis
gcloud redis instances create osprey-cache \
  --size=5 \
  --region=us-central1
```

---

## Checklist Before Production

- [ ] Run load test with production-like data volume
- [ ] Test with production network topology (separate machines)
- [ ] Verify p99 latency under sustained load
- [ ] Test spike scenario (2x normal load)
- [ ] Monitor database connection pool during test
- [ ] Check for memory growth over extended test
- [ ] Verify error handling under load
- [ ] Test failover scenarios (kill instances during load)
- [ ] Document baseline metrics for future comparison
