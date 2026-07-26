---
name: Mengurangi P99 Latency API Sebesar 60% dengan Read Replica Routing
description: Studi kasus teknis menurunkan P99 latency endpoint pencarian dari 900ms ke 340ms dengan memisahkan jalur baca dan tulis database lewat read replica.
tag: database, performance, latency, read-replica, engineering
---

# Mengurangi P99 Latency API Sebesar 60% dengan Read Replica Routing

Tim engineering kami menghabiskan kuartal terakhir menangani satu keluhan yang terus berulang di dashboard support: endpoint pencarian produk lambat saat jam sibuk, meski beban server terlihat normal di monitoring dasar. Tulisan ini menjelaskan apa yang ternyata jadi akar masalahnya, dan bagaimana kami menyelesaikannya tanpa menambah satu server pun.

## Gejala yang terlihat

P99 latency endpoint `/search` naik dari 180ms ke lebih dari 900ms setiap pukul 10 pagi hingga 2 siang waktu server — jam paling ramai untuk basis pengguna kami. CPU utilization database tidak menunjukkan tanda kelebihan beban yang jelas, rata-rata masih di bawah 70%. Ini yang membuat masalahnya tidak langsung kelihatan: metrik agregat terlihat baik-baik saja, padahal pengalaman aktual pengguna memburuk drastis di jam tertentu.

Kami mulai dengan asumsi paling umum — mungkin query-nya tidak efisien. Setelah audit query plan, ternyata bukan itu. Query pencarian sudah pakai index yang tepat, execution time-nya konsisten di bawah 15ms bahkan saat jam sibuk.

## Akar masalah: lock contention, bukan query lambat

Yang sebenarnya terjadi adalah lock contention antara query pencarian (read-heavy) dan proses batch update inventori yang berjalan tiap beberapa menit (write-heavy), keduanya mengarah ke tabel `products` yang sama di database primary. Saat batch update sedang menulis, query pencarian harus menunggu giliran meski query itu sendiri cepat — waktu tunggu inilah yang muncul sebagai latency tinggi di sisi klien, padahal execution time query yang tercatat di query plan tetap rendah.

Ini kelas masalah yang gampang terlewat kalau monitoring hanya melihat rata-rata, bukan distribusi. P50 kami memang tetap baik. Yang meledak adalah P99 — request yang kebetulan datang persis saat lock sedang dipegang proses batch.

## Solusi: pisahkan jalur baca dan tulis

Database kami sudah punya read replica yang sebelumnya cuma dipakai untuk reporting internal. Solusinya adalah mengarahkan seluruh trafik `/search` ke replica, dan menyisakan primary khusus untuk write path (checkout, update inventori, dan operasi tulis lainnya).

Perubahan intinya di connection pool layer:

```python
def get_db_connection(operation_type):
    if operation_type == "read":
        return replica_pool.get_connection()
    return primary_pool.get_connection()
```

Sederhana di permukaan, tapi ada dua hal yang perlu dipikirkan matang sebelum diterapkan ke production:

- **Replication lag.** Replica kami rata-rata tertinggal 200-400ms dari primary. Untuk pencarian produk ini bisa diterima — pengguna tidak akan menyadari selisih ratusan milidetik pada data katalog. Untuk data yang butuh konsistensi ketat seperti status pembayaran, ini tetap harus lewat primary.
- **Health check pada replica.** Kalau replica down atau lag-nya melonjak di atas ambang tertentu, sistem perlu fallback otomatis ke primary daripada menyajikan data yang terlalu basi atau error ke pengguna.

## Hasilnya

Setelah perubahan ini di-rollout penuh, P99 latency endpoint pencarian turun dari 900ms menjadi rata-rata 340ms di jam sibuk yang sama. Lock contention di primary juga berkurang drastis karena beban read yang sebelumnya bersaing dengan write sekarang sudah dipisah jalurnya.

Yang menarik, perbaikan ini tidak terlihat sama sekali kalau kami hanya mengandalkan CPU utilization sebagai sinyal. Pelajaran yang kami bawa: distribusi latency (P95, P99) sering menceritakan masalah yang tidak pernah muncul di rata-rata atau di metrik resource dasar.

## Langkah berikutnya

Kami sedang mengevaluasi penambahan replica kedua khusus untuk trafik pencarian saat kampanye promosi besar, saat volume read bisa naik lima kali lipat dari hari biasa. Kalau ada pembaruan yang layak dibagikan dari eksperimen itu, kami akan tulis di sini.
