/**
 * Osprey Production Load Test
 *
 * This test simulates realistic production traffic patterns:
 * 1. Ramp-up: Gradually increase load
 * 2. Sustained: Hold at target load
 * 3. Spike: Sudden traffic burst
 * 4. Cool-down: Gradual decrease
 *
 * Usage:
 *   # Run against local Docker stack
 *   k6 run k6/production-load-test.js
 *
 *   # Run against remote server
 *   k6 run -e BASE_URL=https://osprey.example.com k6/production-load-test.js
 *
 *   # Run with custom VU count
 *   k6 run -e MAX_VUS=200 k6/production-load-test.js
 *
 *   # Output to InfluxDB for Grafana dashboards
 *   k6 run --out influxdb=http://localhost:8086/k6 k6/production-load-test.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// Custom metrics
const evaluationDuration = new Trend('osprey_evaluation_duration', true);
const alertRate = new Rate('osprey_alert_rate');
const errorCounter = new Counter('osprey_errors');

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const MAX_VUS = parseInt(__ENV.MAX_VUS) || 100;
const TENANT_ID = __ENV.TENANT_ID || 'load-test';

// Production-like traffic pattern
export const options = {
  scenarios: {
    // Phase 1: Ramp-up (simulate morning traffic increase)
    ramp_up: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '30s', target: Math.floor(MAX_VUS * 0.25) },  // 25% load
        { duration: '30s', target: Math.floor(MAX_VUS * 0.5) },   // 50% load
        { duration: '30s', target: Math.floor(MAX_VUS * 0.75) },  // 75% load
        { duration: '30s', target: MAX_VUS },                      // 100% load
      ],
      gracefulRampDown: '10s',
      startTime: '0s',
    },

    // Phase 2: Sustained load (peak hours)
    sustained_load: {
      executor: 'constant-vus',
      vus: MAX_VUS,
      duration: '3m',
      startTime: '2m',  // Start after ramp-up
    },

    // Phase 3: Spike test (flash sale, viral event)
    spike_test: {
      executor: 'ramping-vus',
      startVUs: MAX_VUS,
      stages: [
        { duration: '10s', target: MAX_VUS * 2 },   // Double the load
        { duration: '30s', target: MAX_VUS * 2 },   // Hold spike
        { duration: '10s', target: MAX_VUS },       // Back to normal
      ],
      gracefulRampDown: '10s',
      startTime: '5m',  // Start after sustained
    },

    // Phase 4: Cool-down (end of day)
    cool_down: {
      executor: 'ramping-vus',
      startVUs: MAX_VUS,
      stages: [
        { duration: '30s', target: Math.floor(MAX_VUS * 0.5) },
        { duration: '30s', target: Math.floor(MAX_VUS * 0.25) },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
      startTime: '6m',
    },
  },

  thresholds: {
    // Response time SLAs
    http_req_duration: [
      'p(50)<10',    // 50% under 10ms
      'p(95)<50',    // 95% under 50ms
      'p(99)<100',   // 99% under 100ms
    ],

    // Availability SLA
    http_req_failed: ['rate<0.001'],  // 99.9% success rate

    // Custom metric thresholds
    osprey_evaluation_duration: ['p(95)<50'],
    osprey_errors: ['count<10'],
  },
};

// Transaction types weighted by real-world frequency
const TX_TYPES = [
  { type: 'PAYMENT', weight: 40 },
  { type: 'TRANSFER', weight: 35 },
  { type: 'CASH_OUT', weight: 15 },
  { type: 'CASH_IN', weight: 8 },
  { type: 'DEBIT', weight: 2 },
];

// Amount distribution (log-normal-ish)
function generateAmount() {
  const r = Math.random();
  if (r < 0.6) return randomIntBetween(10, 500);           // 60% small
  if (r < 0.9) return randomIntBetween(500, 5000);         // 30% medium
  if (r < 0.98) return randomIntBetween(5000, 50000);      // 8% large
  return randomIntBetween(50000, 500000);                   // 2% very large
}

// Select transaction type based on weights
function selectTxType() {
  const total = TX_TYPES.reduce((sum, t) => sum + t.weight, 0);
  let r = Math.random() * total;
  for (const t of TX_TYPES) {
    r -= t.weight;
    if (r <= 0) return t.type;
  }
  return 'TRANSFER';
}

// Generate realistic user IDs
function generateUserId() {
  return `user-${randomIntBetween(1, 100000)}`;
}

// Generate realistic account IDs
function generateAccountId() {
  return `acc-${randomIntBetween(1, 200000)}`;
}

// Main test function
export default function() {
  const startTime = Date.now();

  // Build transaction payload
  const debtorId = generateUserId();
  const creditorId = generateUserId();

  // 0.1% chance of same-account transfer (suspicious)
  const isSameAccount = Math.random() < 0.001;

  const payload = {
    type: selectTxType(),
    debtor: {
      id: debtorId,
      accountId: generateAccountId(),
    },
    creditor: {
      id: isSameAccount ? debtorId : creditorId,
      accountId: generateAccountId(),
    },
    amount: {
      value: generateAmount(),
      currency: 'USD',
    },
    metadata: {
      source: 'k6-load-test',
      timestamp: new Date().toISOString(),
    },
  };

  // Send request
  const res = http.post(`${BASE_URL}/evaluate`, JSON.stringify(payload), {
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': TENANT_ID,
    },
    tags: { name: 'evaluate' },
  });

  // Record custom metrics
  const duration = Date.now() - startTime;
  evaluationDuration.add(duration);

  // Validate response
  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'has evaluationId': (r) => {
      try {
        return JSON.parse(r.body).evaluationId !== undefined;
      } catch {
        return false;
      }
    },
    'response time < 100ms': (r) => r.timings.duration < 100,
  });

  // Only log actual failures (non-200 responses)
  if (res.status !== 200) {
    errorCounter.add(1);
    console.error(`Request failed: ${res.status} - ${res.body}`);
  }

  // Track alert rate
  try {
    const body = JSON.parse(res.body);
    alertRate.add(body.status === 'ALRT');
  } catch {
    // Ignore parse errors
  }

  // Simulate realistic think time (user behavior)
  // Production users don't hammer continuously
  sleep(randomIntBetween(1, 3) / 10);  // 0.1-0.3s think time
}

// Setup: Verify server is healthy before test
export function setup() {
  console.log(`\n========================================`);
  console.log(`  OSPREY PRODUCTION LOAD TEST`);
  console.log(`========================================`);
  console.log(`  Target:     ${BASE_URL}`);
  console.log(`  Max VUs:    ${MAX_VUS}`);
  console.log(`  Tenant:     ${TENANT_ID}`);
  console.log(`========================================\n`);

  const healthCheck = http.get(`${BASE_URL}/health`);
  const healthy = check(healthCheck, {
    'server is healthy': (r) => r.status === 200,
  });

  if (!healthy) {
    console.error('Server health check failed!');
    console.error(`Status: ${healthCheck.status}`);
    console.error(`Body: ${healthCheck.body}`);
    throw new Error('Server not healthy');
  }

  try {
    const healthData = JSON.parse(healthCheck.body);
    console.log(`Server healthy:`);
    console.log(`  Version: ${healthData.version}`);
    console.log(`  Mode:    ${healthData.mode}`);
    console.log(`  Status:  ${healthData.status}\n`);
  } catch {
    // Ignore parse errors
  }

  return { startTime: Date.now() };
}

// Teardown: Print summary
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`\n========================================`);
  console.log(`  TEST COMPLETE`);
  console.log(`========================================`);
  console.log(`  Duration: ${duration.toFixed(1)}s`);
  console.log(`========================================\n`);
}

// Handle summary output
export function handleSummary(data) {
  const summary = {
    timestamp: new Date().toISOString(),
    duration_seconds: data.state.testRunDurationMs / 1000,
    metrics: {
      requests: {
        total: data.metrics.http_reqs?.values?.count || 0,
        rate: data.metrics.http_reqs?.values?.rate || 0,
        failed: data.metrics.http_req_failed?.values?.rate || 0,
      },
      latency: {
        avg: data.metrics.http_req_duration?.values?.avg || 0,
        p50: data.metrics.http_req_duration?.values['p(50)'] || 0,
        p95: data.metrics.http_req_duration?.values['p(95)'] || 0,
        p99: data.metrics.http_req_duration?.values['p(99)'] || 0,
        max: data.metrics.http_req_duration?.values?.max || 0,
      },
      custom: {
        alert_rate: data.metrics.osprey_alert_rate?.values?.rate || 0,
        errors: data.metrics.osprey_errors?.values?.count || 0,
      },
    },
    thresholds_passed: Object.values(data.root_group?.checks || {})
      .every(c => c.passes === c.passes + c.fails),
  };

  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'k6/load-test-results.json': JSON.stringify(summary, null, 2),
  };
}

// Generate text summary
function textSummary(data, options) {
  const metrics = data.metrics;

  let output = '\n';
  output += '╔══════════════════════════════════════════════════════════════╗\n';
  output += '║              OSPREY PRODUCTION LOAD TEST RESULTS             ║\n';
  output += '╚══════════════════════════════════════════════════════════════╝\n\n';

  output += '📊 THROUGHPUT\n';
  output += `   Total Requests:  ${metrics.http_reqs?.values?.count || 0}\n`;
  output += `   Requests/sec:    ${(metrics.http_reqs?.values?.rate || 0).toFixed(2)}\n`;
  output += `   Failed:          ${((metrics.http_req_failed?.values?.rate || 0) * 100).toFixed(3)}%\n\n`;

  output += '⏱️  LATENCY\n';
  output += `   Average:         ${(metrics.http_req_duration?.values?.avg || 0).toFixed(2)}ms\n`;
  output += `   Median (p50):    ${(metrics.http_req_duration?.values['p(50)'] || 0).toFixed(2)}ms\n`;
  output += `   p95:             ${(metrics.http_req_duration?.values['p(95)'] || 0).toFixed(2)}ms\n`;
  output += `   p99:             ${(metrics.http_req_duration?.values['p(99)'] || 0).toFixed(2)}ms\n`;
  output += `   Max:             ${(metrics.http_req_duration?.values?.max || 0).toFixed(2)}ms\n\n`;

  output += '🎯 OSPREY METRICS\n';
  output += `   Alert Rate:      ${((metrics.osprey_alert_rate?.values?.rate || 0) * 100).toFixed(2)}%\n`;
  output += `   Errors:          ${metrics.osprey_errors?.values?.count || 0}\n\n`;

  return output;
}
