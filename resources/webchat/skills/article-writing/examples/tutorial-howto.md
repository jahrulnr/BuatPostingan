---
name: Cara Setup Rate Limiting dengan Redis di Express.js
description: Panduan langkah demi langkah memasang rate limiting sliding window berbasis Redis di Express.js, lengkap kode dan cara mengetesnya.
tag: redis, express-js, rate-limiting, nodejs, tutorial, api
---

# Cara Setup Rate Limiting dengan Redis di Express.js

Kalau API kamu pernah down karena satu klien nge-spam request ratusan kali per detik, tutorial ini buat kamu. Kita akan pasang rate limiting berbasis [Redis](https://redis.io) di aplikasi [Express.js](https://expressjs.com), pakai algoritma sliding window supaya pembatasannya lebih akurat dibanding fixed window biasa.

Kenapa Redis dan bukan in-memory store bawaan? Kalau aplikasi kamu jalan di lebih dari satu instance (yang hampir pasti terjadi begitu kamu scale horizontal), rate limit yang disimpan di memori tiap instance jadi tidak konsisten — klien bisa saja "reset" limit-nya cuma dengan kena instance yang berbeda. Redis jadi satu sumber kebenaran yang dibagi semua instance.

## Prasyarat

Sebelum mulai, pastikan kamu sudah punya:

- Node.js versi 18 ke atas
- Redis server yang jalan (lokal atau managed, misalnya lewat Docker: `docker run -d -p 6379:6379 redis`)
- Aplikasi Express.js yang sudah berjalan (tutorial ini asumsikan kamu sudah punya struktur dasar `app.js` atau `index.js`)

## Langkah 1: Install dependency

```bash
npm install express redis ioredis
```

Kita pakai [`ioredis`](https://www.npmjs.com/package/ioredis) daripada `redis` client bawaan karena API-nya lebih ergonomis untuk operasi atomic yang kita butuhkan nanti.

## Langkah 2: Buat koneksi Redis

Buat file `redis-client.js`:

```javascript
const Redis = require('ioredis');

const redis = new Redis({
  host: process.env.REDIS_HOST || 'localhost',
  port: process.env.REDIS_PORT || 6379,
  maxRetriesPerRequest: 3,
});

redis.on('error', (err) => {
  console.error('Redis connection error:', err);
});

module.exports = redis;
```

Poin penting di sini: `maxRetriesPerRequest` dibatasi supaya kalau Redis down, request tidak menggantung lama menunggu retry — API kamu tetap harus punya perilaku yang jelas (biasanya fail-open, izinkan request lewat) saat Redis tidak bisa diakses, daripada ikut down bareng.

## Langkah 3: Implementasi sliding window counter

Buat file `rate-limiter.js`:

```javascript
const redis = require('./redis-client');

async function checkRateLimit(key, limit, windowSeconds) {
  const now = Date.now();
  const windowStart = now - windowSeconds * 1000;

  const pipeline = redis.pipeline();
  pipeline.zremrangebyscore(key, 0, windowStart);
  pipeline.zadd(key, now, `${now}-${Math.random()}`);
  pipeline.zcard(key);
  pipeline.expire(key, windowSeconds);

  const results = await pipeline.exec();
  const requestCount = results[2][1];

  return {
    allowed: requestCount <= limit,
    remaining: Math.max(0, limit - requestCount),
    count: requestCount,
  };
}

module.exports = { checkRateLimit };
```

Yang terjadi di sini: kita pakai [Redis sorted set](https://redis.io/docs/latest/develop/data-types/sorted-sets/), di mana setiap request dicatat dengan timestamp sebagai score. Setiap kali ada request baru, kita hapus dulu entri yang sudah lewat dari window waktu (`zremrangebyscore`), lalu tambah entri baru, lalu hitung berapa banyak yang masih ada di dalam window (`zcard`). Karena semua operasi ini dibungkus dalam satu pipeline, hasilnya atomic — tidak ada race condition antar request yang datang bersamaan.

## Langkah 4: Buat middleware Express

Buat file `middleware/rateLimitMiddleware.js`:

```javascript
const { checkRateLimit } = require('../rate-limiter');

function rateLimit({ limit = 100, windowSeconds = 60 } = {}) {
  return async (req, res, next) => {
    const identifier = req.ip;
    const key = `ratelimit:${identifier}`;

    try {
      const result = await checkRateLimit(key, limit, windowSeconds);

      res.set('X-RateLimit-Limit', limit);
      res.set('X-RateLimit-Remaining', result.remaining);

      if (!result.allowed) {
        return res.status(429).json({
          error: 'Terlalu banyak request, coba lagi nanti.',
        });
      }

      next();
    } catch (err) {
      console.error('Rate limit check failed:', err);
      next();
    }
  };
}

module.exports = rateLimit;
```

Perhatikan blok `catch` di bagian bawah: kalau Redis gagal diakses karena alasan apa pun, kita panggil `next()` tetap, bukan blokir request. Ini keputusan sadar — kegagalan infrastruktur pendukung (rate limiter) sebaiknya tidak ikut menjatuhkan fungsi utama API.

## Langkah 5: Pasang di aplikasi

```javascript
const express = require('express');
const rateLimit = require('./middleware/rateLimitMiddleware');

const app = express();

app.use('/api/', rateLimit({ limit: 100, windowSeconds: 60 }));

app.get('/api/products', (req, res) => {
  res.json({ products: [] });
});

app.listen(3000, () => console.log('Server jalan di port 3000'));
```

## Langkah 6: Tes rate limiting-nya

Kirim beberapa request beruntun untuk memastikan limit-nya kena:

```bash
for i in {1..105}; do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000/api/products; done
```

Kamu akan melihat 100 request pertama balas `200`, lalu sisanya `429`. Kalau semuanya `200`, cek dulu apakah `REDIS_HOST` sudah benar dan koneksi ke Redis berhasil — lihat log error di terminal.

## Catatan tambahan

Rate limit berbasis IP (`req.ip`) cukup untuk kasus umum, tapi kalau API kamu punya sistem autentikasi, sebaiknya limit berdasarkan user ID atau API key, bukan IP saja — satu kantor bisa berbagi IP yang sama lewat NAT, dan itu bisa membuat banyak pengguna sah kena limit bersama-sama gara-gara satu klien yang nakal.
