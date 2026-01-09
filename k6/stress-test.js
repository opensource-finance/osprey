import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Custom metrics
const errorRate = new Rate('errors');
const alertRate = new Rate('alerts_triggered');
const evaluationDuration = new Trend('evaluation_duration', true);
const transactionsProcessed = new Counter('transactions_processed');

// Configuration
const BASE_URL = __ENV.OSPREY_URL || 'http://localhost:8080';
const TENANT_ID = __ENV.TENANT_ID || 'k6-stress-test';

// Test scenarios
export const options = {
  scenarios: {
    // Smoke test - verify system works under minimal load
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      startTime: '0s',
      tags: { test_type: 'smoke' },
    },

    // Load test - normal expected load
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 20 },   // Ramp up to 20 users
        { duration: '3m', target: 20 },   // Stay at 20 users
        { duration: '1m', target: 0 },    // Ramp down
      ],
      startTime: '35s',
      tags: { test_type: 'load' },
    },

    // Stress test - beyond normal capacity
    stress: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 50 },   // Ramp up
        { duration: '2m', target: 50 },   // Stay at stress level
        { duration: '1m', target: 100 },  // Push further
        { duration: '2m', target: 100 },  // Hold
        { duration: '1m', target: 0 },    // Ramp down
      ],
      startTime: '6m',
      tags: { test_type: 'stress' },
    },

    // Spike test - sudden traffic burst
    spike: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 5 },    // Warm up
        { duration: '10s', target: 200 },  // Spike!
        { duration: '30s', target: 200 },  // Hold spike
        { duration: '10s', target: 5 },    // Scale down
        { duration: '30s', target: 5 },    // Recovery
      ],
      startTime: '13m',
      tags: { test_type: 'spike' },
    },

    // Breakpoint test - find the breaking point
    breakpoint: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 500,
      maxVUs: 1000,
      stages: [
        { duration: '2m', target: 50 },   // 50 req/s
        { duration: '2m', target: 100 },  // 100 req/s
        { duration: '2m', target: 200 },  // 200 req/s
        { duration: '2m', target: 500 },  // 500 req/s
        { duration: '2m', target: 1000 }, // 1000 req/s - breaking point?
      ],
      startTime: '15m',
      tags: { test_type: 'breakpoint' },
    },
  },

  // Thresholds - test fails if these aren't met
  thresholds: {
    http_req_duration: [
      'p(95)<500',  // 95% of requests under 500ms
      'p(99)<1000', // 99% of requests under 1s
    ],
    http_req_failed: ['rate<0.01'],     // Less than 1% failures
    errors: ['rate<0.05'],               // Less than 5% errors
    evaluation_duration: ['p(95)<400'],  // Evaluation under 400ms
  },
};

// Transaction types with realistic distributions
const TRANSACTION_TYPES = [
  { type: 'PAYMENT', weight: 40 },
  { type: 'TRANSFER', weight: 30 },
  { type: 'CASH_OUT', weight: 20 },
  { type: 'CASH_IN', weight: 8 },
  { type: 'DEBIT', weight: 2 },
];

// Generate realistic transaction payload
function generateTransaction() {
  // Pick transaction type based on weight
  const rand = Math.random() * 100;
  let cumulative = 0;
  let txType = 'PAYMENT';
  for (const t of TRANSACTION_TYPES) {
    cumulative += t.weight;
    if (rand <= cumulative) {
      txType = t.type;
      break;
    }
  }

  // Generate amounts with realistic distribution
  // Most transactions are small, few are large
  let amount;
  const amountRand = Math.random();
  if (amountRand < 0.7) {
    amount = randomIntBetween(10, 1000);           // 70%: small
  } else if (amountRand < 0.9) {
    amount = randomIntBetween(1000, 10000);        // 20%: medium
  } else if (amountRand < 0.98) {
    amount = randomIntBetween(10000, 100000);      // 8%: large
  } else {
    amount = randomIntBetween(100000, 1000000);    // 2%: very large
  }

  // Generate balances
  const oldBalance = randomIntBetween(0, 500000);

  // Simulate account drain scenario (10% chance)
  const isDrain = Math.random() < 0.1;
  const newBalance = isDrain ? 0 : Math.max(0, oldBalance - amount);

  return {
    transaction_id: `k6-${Date.now()}-${randomIntBetween(1, 999999)}`,
    type: txType,
    amount: amount,
    currency: 'USD',
    timestamp: new Date().toISOString(),
    debtor: {
      id: `C${randomIntBetween(100000000, 999999999)}`,
      name: 'K6 Test Debtor',
      account: `ACC${randomIntBetween(10000, 99999)}`,
    },
    creditor: {
      id: `C${randomIntBetween(100000000, 999999999)}`,
      name: 'K6 Test Creditor',
      account: `ACC${randomIntBetween(10000, 99999)}`,
    },
    metadata: {
      old_balance: oldBalance,
      new_balance: newBalance,
      channel: 'mobile',
      device_id: `device-${randomIntBetween(1000, 9999)}`,
    },
  };
}

// Health check
export function setup() {
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health check passed': (r) => r.status === 200,
  });

  if (healthRes.status !== 200) {
    throw new Error(`Osprey is not healthy: ${healthRes.status}`);
  }

  console.log(`Starting stress test against ${BASE_URL}`);
  return { startTime: Date.now() };
}

// Main test function
export default function () {
  const payload = generateTransaction();

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': TENANT_ID,
    },
    tags: {
      endpoint: 'evaluate',
    },
  };

  const startTime = Date.now();
  const response = http.post(
    `${BASE_URL}/evaluate`,
    JSON.stringify(payload),
    params
  );
  const duration = Date.now() - startTime;

  // Record custom metrics
  evaluationDuration.add(duration);
  transactionsProcessed.add(1);

  // Check response
  const success = check(response, {
    'status is 200': (r) => r.status === 200,
    'response has status': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'ALRT' || body.status === 'NALT';
      } catch {
        return false;
      }
    },
    'response has score': (r) => {
      try {
        const body = JSON.parse(r.body);
        return typeof body.score === 'number';
      } catch {
        return false;
      }
    },
    'response time OK': (r) => r.timings.duration < 500,
  });

  // Track errors
  errorRate.add(!success);

  // Track alert rate
  if (response.status === 200) {
    try {
      const body = JSON.parse(response.body);
      alertRate.add(body.status === 'ALRT');
    } catch {
      // ignore parse errors
    }
  }

  // Small sleep to simulate realistic user behavior
  sleep(randomIntBetween(1, 3) / 10); // 0.1-0.3s
}

// Summary output
export function handleSummary(data) {
  const summary = {
    timestamp: new Date().toISOString(),
    test_duration: data.state.testRunDurationMs,
    scenarios_run: Object.keys(options.scenarios),

    // Request metrics
    total_requests: data.metrics.http_reqs?.values?.count || 0,
    failed_requests: data.metrics.http_req_failed?.values?.rate || 0,

    // Latency percentiles
    latency: {
      avg: data.metrics.http_req_duration?.values?.avg || 0,
      min: data.metrics.http_req_duration?.values?.min || 0,
      max: data.metrics.http_req_duration?.values?.max || 0,
      p50: data.metrics.http_req_duration?.values['p(50)'] || 0,
      p90: data.metrics.http_req_duration?.values['p(90)'] || 0,
      p95: data.metrics.http_req_duration?.values['p(95)'] || 0,
      p99: data.metrics.http_req_duration?.values['p(99)'] || 0,
    },

    // Throughput
    requests_per_second: data.metrics.http_reqs?.values?.rate || 0,

    // Custom metrics
    transactions_processed: data.metrics.transactions_processed?.values?.count || 0,
    error_rate: data.metrics.errors?.values?.rate || 0,
    alert_rate: data.metrics.alerts_triggered?.values?.rate || 0,

    // Thresholds
    thresholds_passed: !Object.values(data.metrics).some(m => m.thresholds && Object.values(m.thresholds).some(t => !t.ok)),
  };

  console.log('\n' + '='.repeat(60));
  console.log('OSPREY STRESS TEST SUMMARY');
  console.log('='.repeat(60));
  console.log(`Total Requests:     ${summary.total_requests.toLocaleString()}`);
  console.log(`Requests/sec:       ${summary.requests_per_second.toFixed(2)}`);
  console.log(`Error Rate:         ${(summary.error_rate * 100).toFixed(2)}%`);
  console.log(`Alert Rate:         ${(summary.alert_rate * 100).toFixed(2)}%`);
  console.log('');
  console.log('LATENCY PERCENTILES:');
  console.log(`  p50:  ${summary.latency.p50.toFixed(2)}ms`);
  console.log(`  p90:  ${summary.latency.p90.toFixed(2)}ms`);
  console.log(`  p95:  ${summary.latency.p95.toFixed(2)}ms`);
  console.log(`  p99:  ${summary.latency.p99.toFixed(2)}ms`);
  console.log(`  max:  ${summary.latency.max.toFixed(2)}ms`);
  console.log('');
  console.log(`Thresholds Passed:  ${summary.thresholds_passed ? 'YES' : 'NO'}`);
  console.log('='.repeat(60));

  return {
    'stdout': JSON.stringify(summary, null, 2),
    'k6/stress-test-results.json': JSON.stringify(summary, null, 2),
  };
}
