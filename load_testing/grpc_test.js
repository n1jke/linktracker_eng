import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
const client = new grpc.Client();
client.load(['./../api/grpc/'], 'link_tracker.proto');

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

const GRPC_ADDR = 'localhost:8080'; 
const USERS_RANGE = 1000; 
const BASE_URL = "https://github.com/n1jke/";

export default function () {
  if (__ITER === 0) {
    client.connect(GRPC_ADDR, { plaintext: true });
  }

  const chatId = Math.floor(Math.random() * USERS_RANGE) + 1;
  const chance = Math.random();

  // 99% - read, 1% - write
  if (chance <= 0.01) {
    const linkIdx = Math.floor(Math.random() * 100) + 100;
    const url = `${BASE_URL}${linkIdx}`;

    if (Math.random() > 0.5) {
      const response = client.invoke('linktracker.ScrapperService/TrackLink', {
        chat_id: chatId,
        url: url,
        tags: ["load-test"]
      });

      check(response, {
        'track status is success': (r) =>
          r && (r.status === grpc.StatusOK || r.status === grpc.StatusAlreadyExists),
      });
    } else {
      const response = client.invoke('linktracker.ScrapperService/UntrackLink', {
        chat_id: chatId,
        url: url
      });

      check(response, {
        'untrack status is success': (r) =>
          r.status === grpc.StatusOK || r.status === grpc.StatusNotFound,
      });
    }
  } else {
    const response = client.invoke('linktracker.ScrapperService/ListLinks', { chat_id: chatId });

    check(response, {
      'list status is OK': (r) => r.status === grpc.StatusOK,
    });
  }

  sleep(0.01);
}
