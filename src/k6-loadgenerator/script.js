import http from 'k6/http';
import { sleep } from 'k6';

// Base URL of the service under test
const BASE_URL = __ENV.BASE_URL || 'http://frontend:80';

function formEncode(obj) {
    return Object.entries(obj)
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
        .join('&');
}

// Sample product IDs
const products = [
    '0PUK6V6EV0', '1YMWWN1N4O', '2ZYFJ3GM2N', '66VCHSJNUP',
    '6E92ZMYYFZ', '9SIQT8TOJO', 'L9ECAV7KIM', 'LS4PSXUNUM', 'OLJCESPC7Z'
];

// Supported currencies
const currencies = ['EUR', 'USD', 'JPY', 'CAD'];

// Define tasks with weights
const tasks = [
    { fn: index, weight: 1 },
    { fn: setCurrency, weight: 1 },
    { fn: browseProduct, weight: 1 },
    { fn: addToCart, weight: 1 },
    { fn: viewCart, weight: 1 },
    { fn: checkout, weight: 1 },
];

// k6 options
export const options = {
    vus: __ENV.USERS ? parseInt(__ENV.USERS) : 10,
    duration: __ENV.DURATION || '1m',
};

// Utility: pick a task based on weight
function pickTask() {
    const total = tasks.reduce((sum, t) => sum + t.weight, 0);
    let r = Math.random() * total;
    for (const t of tasks) {
        if (r < t.weight) {
            return t.fn;
        }
        r -= t.weight;
    }
    return tasks[0].fn;
}

// Task definitions
function index() {
    http.get(`${BASE_URL}/`);
}

function setCurrency() {
    const data = {
        currency_code: currencies[Math.floor(Math.random() * currencies.length)],
    };
    const payload = formEncode(data);
    http.post(
        `${BASE_URL}/setCurrency`,
        payload,
        { headers: { 'Content-Type': 'application/x-www-form-urlencoded' } }
    );
}

function browseProduct() {
    const id = products[Math.floor(Math.random() * products.length)];
    http.get(`${BASE_URL}/product/${id}`);
}

function viewCart() {
    http.get(`${BASE_URL}/cart`);
}

function addToCart() {
    const id = products[Math.floor(Math.random() * products.length)];
    // 商品詳細取得はGETのまま
    http.get(`${BASE_URL}/product/${id}`);

    const data = {
        product_id: id,
        quantity: [1,2,3,4,5,10][Math.floor(Math.random() * 6)],
    };
    const payload = formEncode(data);
    http.post(
        `${BASE_URL}/cart`,
        payload,
        { headers: { 'Content-Type': 'application/x-www-form-urlencoded' } }
    );
}

function checkout() {
    addToCart();

    const data = {
        email: 'someone@example.com',
        street_address: '1600 Amphitheatre Parkway',
        zip_code: '94043',
        city: 'Mountain View',
        state: 'CA',
        country: 'United States',
        credit_card_number: '4432-8015-6152-0454',
        credit_card_expiration_month: '1',
        credit_card_expiration_year: '2039',
        credit_card_cvv: '672',
    };
    const payload = formEncode(data);
    http.post(
        `${BASE_URL}/cart/checkout`,
        payload,
        { headers: { 'Content-Type': 'application/x-www-form-urlencoded' } }
    );

    sleep(1);
}
// Default function that runs each VU iteration
export default function () {
    const taskFn = pickTask();
    taskFn();
    sleep(Math.random() * 9 + 1); // wait between 1 and 10 seconds
}
