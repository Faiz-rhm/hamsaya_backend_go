// k6 load profile for the post-create write path.
//
// Run:
//   API_URL=http://localhost:8080 ACCESS_TOKEN=eyJ... \
//     k6 run tests/load/post_create.js
//
// Note: this test is bounded by the per-user rate limit on POST /posts
// (30/hour). For a representative write benchmark, mint a pool of access
// tokens (one per VU) and pass them as a JSON file via __ENV.TOKENS_FILE,
// or remove the rate limit in a dedicated load environment.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const apiUrl = __ENV.API_URL || 'http://localhost:8080';
const token = __ENV.ACCESS_TOKEN;

if (!token) {
  throw new Error('ACCESS_TOKEN env var is required');
}

const writeLatency = new Trend('post_create_latency_ms', true);
const errorRate = new Rate('post_create_error_rate');

export const options = {
  stages: [
    { duration: '30s', target: 5 },
    { duration: '2m', target: 5 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    'post_create_latency_ms': ['p(95)<800'],
    'post_create_error_rate': ['rate<0.05'], // allow rate-limit 429s
  },
};

export default function () {
  const body = JSON.stringify({
    type: 'feed',
    description: `k6 load test post ${__VU}/${__ITER} @ ${Date.now()}`,
    visibility: 'public',
  });

  const res = http.post(`${apiUrl}/api/v1/posts`, body, {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    tags: { endpoint: 'post_create' },
  });

  writeLatency.add(res.timings.duration);
  const ok = check(res, {
    // 201 success or 429 rate-limit are both expected.
    'status accepted': (r) => r.status === 201 || r.status === 429,
  });
  errorRate.add(!ok);

  sleep(2);
}
