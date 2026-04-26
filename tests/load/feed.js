// k6 load profile for the personalized feed read path.
//
// Run:
//   API_URL=http://localhost:8080 ACCESS_TOKEN=eyJ... \
//     k6 run tests/load/feed.js
//
// Stages ramp to 50 VUs over 1 minute, hold for 3 minutes, ramp down. Adjust
// for the target environment — these defaults are for a local dev stack.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const apiUrl = __ENV.API_URL || 'http://localhost:8080';
const token = __ENV.ACCESS_TOKEN;

if (!token) {
  throw new Error('ACCESS_TOKEN env var is required');
}

const feedLatency = new Trend('feed_latency_ms', true);
const errorRate = new Rate('feed_error_rate');

export const options = {
  stages: [
    { duration: '1m', target: 50 },   // ramp up
    { duration: '3m', target: 50 },   // sustained load
    { duration: '30s', target: 0 },   // ramp down
  ],
  thresholds: {
    // p95 must stay under 400ms; error rate under 1%.
    'feed_latency_ms': ['p(95)<400'],
    'feed_error_rate': ['rate<0.01'],
  },
};

export default function () {
  const res = http.get(`${apiUrl}/api/v1/posts/feed?limit=20`, {
    headers: { Authorization: `Bearer ${token}` },
    tags: { endpoint: 'feed' },
  });

  feedLatency.add(res.timings.duration);
  const ok = check(res, {
    'status 200': (r) => r.status === 200,
    'has data': (r) => r.json('data') !== null,
  });
  errorRate.add(!ok);

  sleep(1);
}
