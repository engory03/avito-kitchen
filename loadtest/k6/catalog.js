import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const profile = __ENV.LOADTEST_PROFILE || '';
const rate = Number(__ENV.LOADTEST_RATE || '50');
const duration = __ENV.LOADTEST_DURATION || '90s';

export const options = profile === 'smoke'
  ? { vus: 1, duration: '10s' }
  : {
      scenarios: {
        catalog: {
          executor: 'constant-arrival-rate',
          rate,
          timeUnit: '1s',
          duration,
          preAllocatedVUs: Math.min(rate, 20),
        },
      },
    };

export default function () {
  const health = http.get(`${base}/health`);
  check(health, { 'health 200': (r) => r.status === 200 });

  const list = http.get(`${base}/api/v1/restaurants`);
  check(list, { 'restaurants 200': (r) => r.status === 200 });
  sleep(0.1);
}
