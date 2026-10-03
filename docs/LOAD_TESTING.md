# Load Testing

Use this guide to measure the Osprey throughput with traffic that is similar to the traffic of your deployment.

Use local SQLite tests only as smoke checks.
They do not predict the latency of a remote production deployment.
In production, PostgreSQL, Redis, NATS, container limits, and network hops add overhead.

## Quick Test

1. Start the Docker stack:

   ```bash
   OSPREY_ADMIN_TOKEN=local-admin-token docker-compose up -d
   ```

2. Seed the rules:

   ```bash
   OSPREY_ADMIN_TOKEN=local-admin-token ./scripts/seed-rules.sh
   ```

3. Run k6:

   ```bash
   k6 run k6/production-load-test.js
   ```

4. Stop the stack:

   ```bash
   docker-compose down
   ```

To do all of these steps with one command, use the wrapper script:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token ./scripts/load-test.sh docker
```

## Remote Target

To test a remote deployment, give k6 the base URL and the tenant ID:

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

The default k6 script has four phases:

1. It increases the load.
2. It holds a sustained load.
3. It sends a spike.
4. It decreases the load.

## Useful Checks During a Run

Use these commands while the test runs:

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
| p99 is much higher than p95 | Pressure on the connection pool, or slow storage. |
| Latency increases over time | Memory pressure, GC pressure, or queues that grow. |
| Errors during a spike | Too little headroom. |
| PostgreSQL timeouts | Slow queries, locks, or too few connections. |
| Redis latency | Cache pressure or network problems. |

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

## Before You Use the Results

Do these checks before you use the results:

- Use data that is similar to production data.
- If you test a remote deployment, run k6 from a different machine or region.
- Examine the p99 latency under sustained load.
- Send a short spike above the expected peak.
- Monitor the database connections and the container memory.
- Record the baseline command, commit, ruleset, and environment.
