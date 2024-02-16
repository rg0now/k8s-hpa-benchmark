import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
    stages: [
      { duration: '1m', target: 20 },
      { duration: '28m', target: 22 },
      { duration: '1m', target: 1 },
    ],
};

export default function() {
    var addr = 'localhost';
    if (typeof __ENV.CHAINER_IP !== 'undefined') {
        addr = __ENV.CHAINER_IP;
    } else {
        console.error("CHAINER_IP undefined");
    }

    let url = `http://${addr}:8888`;
    let params = {
        timeout: 600
    };

    let start_time = new Date();
    let res = http.get(url, params);
    let end_time = new Date();

    if (typeof res === 'undefined') {
        fail("request timeout");
    }

    let diff = (end_time - start_time) / 1000.0;
    sleep(1 - diff);
}
