import http from 'k6/http';
import { check, sleep } from 'k6';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Quick test - single scenario for fast feedback
export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.OSPREY_URL || 'http://localhost:8080';

function generateTransaction() {
  const types = ['PAYMENT', 'TRANSFER', 'CASH_OUT'];
  const amount = randomIntBetween(100, 50000);
  const oldBalance = randomIntBetween(1000, 100000);

  return {
    type: types[randomIntBetween(0, 2)],
    debtor: {
      id: `C${randomIntBetween(100000, 999999)}`,
      accountId: `ACC${randomIntBetween(1000, 9999)}`,
    },
    creditor: {
      id: `C${randomIntBetween(100000, 999999)}`,
      accountId: `ACC${randomIntBetween(1000, 9999)}`,
    },
    amount: {
      value: amount,
      currency: 'USD',
    },
    metadata: {
      old_balance: oldBalance,
      new_balance: Math.max(0, oldBalance - amount),
    },
  };
}

export default function () {
  const payload = generateTransaction();

  const response = http.post(`${BASE_URL}/evaluate`, JSON.stringify(payload), {
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': 'k6-test-tenant',
    },
  });

  check(response, {
    'status 200': (r) => r.status === 200,
    'valid response': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status && body.score !== undefined;
      } catch {
        return false;
      }
    },
  });

  sleep(0.1);
}
