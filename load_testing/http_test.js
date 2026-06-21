import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    scrapper_load_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 32 }, // Ramp-up
        { duration: '3m', target: 32 }, // Stage
        { duration: '10s', target: 0 }, // Ramp-down
      ],
    },
  },
};

const API_URL = 'http://localhost:8080';
const USERS_RANGE = 1000;
const BASE_URL = "https://github.com/n1jke/";

export default function () {
  const chatId = Math.floor(Math.random() * USERS_RANGE) + 1;
  const chance = Math.random();

  const baseHeaders = {
    'Content-Type': 'application/json',
    'Tg-Chat-Id': chatId.toString(),
  };

  if (chance <= 0.01) {
    const linkIdx = Math.floor(Math.random() * 100) + 100;
    const linkUrl = `${BASE_URL}${linkIdx}`;

    if (Math.random() > 0.5) {
      const payload = JSON.stringify({ link: linkUrl, tags: ["load-test"] });
      const params = { headers: baseHeaders, tags: { name: 'POST /links' } };
      const res = http.post(`${API_URL}/links`, payload, params);

      check(res, {
        'POST status is 200 or 409': (r) => r.status === 200 || r.status === 409,
      });

    } else {
      const payload = JSON.stringify({ link: linkUrl });
      const params = { headers: baseHeaders, body: payload, tags: { name: 'DELETE /links' } };
      const res = http.del(`${API_URL}/links`, payload, params);

      check(res, {
        'DELETE status is 200 or 404': (r) => r.status === 200 || r.status === 404,
      });
    }
  } else {
    const params = { headers: baseHeaders, tags: { name: 'GET /links' } };
    const res = http.get(`${API_URL}/links`, params);

    check(res, {
      'GET status is 200': (r) => r.status === 200,
    });
  }

  sleep(0.01);
}