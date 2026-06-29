# Load Testing

Use this guide to measure Osprey throughput with traffic that looks like your deployment.

Local SQLite tests are useful for smoke checks. They do not predict remote production latency because PostgreSQL, Redis, NATS, container limits, and network hops add overhead.

## Quick Test

Start the Docker stack:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token docker-compose up -d
```

Seed rules:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token ./scripts/seed-rules.sh
```

Run k6:

```bash
k6 run k6/production-load-test.js
```

Stop the stack:

```bash
docker-compose down
```

One-command wrapper:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token ./scripts/load-test.sh docker
```

## Remote Target

```bash
k6 run \
  -e BASE_URL=https://osprey.example.com \
  -e TENANT_ID=load-test \
  k6/production-load-test.js
```

## What to Watch

| Metric | Target |
|--------|--------|
| Failed requests | Below `0.1%` |
| p95 latency | Below your review SLA |
| p99 latency | Stable during sustained load |
| Throughput | Stable during the sustained phase |

The default k6 script ramps up, holds sustained load, spikes, then cools down.

## Useful Checks During a Run

```bash
docker stats
curl -fsS http://localhost:8080/health | jq
docker exec osprey-postgres psql -U osprey -c "SELECT count(*) FROM pg_stat_activity;"
docker exec osprey-redis redis-cli info stats
curl -fsS http://localhost:8222/varz | jq '.connections, .slow_consumers'
```

## Warning Signs

| Symptom | Likely Cause |
|---------|--------------|
| p99 much higher than p95 | Connection-pool pressure or slow storage. |
| Latency increases over time | Memory pressure, GC pressure, or growing queues. |
| Errors during spike | Too little headroom. |
| PostgreSQL timeouts | Slow queries, locks, or too few connections. |
| Redis latency | Cache pressure or network issues. |

## Capacity Formula

```text
instances = (peak TPS * safety margin) / measured TPS per instance
```

Example:

```text
peak TPS = 5000
safety margin = 1.5
measured TPS per instance = 1500

instances = (5000 * 1.5) / 1500 = 5
```

## Before Relying on Results

- Run against production-like data.
- Run from a separate machine or region when testing a remote deployment.
- Verify p99 latency under sustained load.
- Test a short spike above expected peak.
- Watch database connections and container memory.
- Record the baseline command, commit, ruleset, and environment.
