---
name: Kenapa Saya Akhirnya Membongkar Microservices di Startup 6 Orang
description: Cerita nyata membongkar sembilan microservices jadi dua service setelah insiden empat jam, dan pelajaran soal kapan pola arsitektur itu benar-benar dibutuhkan.
tag: microservices, arsitektur-software, startup, refactoring, teknologi
---

# Kenapa Saya Akhirnya Membongkar Microservices di Startup 6 Orang

Dua tahun lalu, sistem kami terdiri dari sembilan service terpisah untuk produk yang penggunanya belum sampai seribu orang. Setiap deploy butuh koordinasi di empat repo berbeda. Satu bug kecil di service auth bisa bikin tiga service lain ikut down, dan butuh waktu setengah jam cuma buat nyari log yang benar karena tersebar di sembilan tempat.

Saya yang mengusulkan arsitektur itu di awal. Jadi tulisan ini bukan cerita orang lain yang salah — ini pengakuan.

## Alasan aslinya, waktu itu

Waktu merancang sistem ini, saya baru selesai baca beberapa artikel tentang bagaimana perusahaan besar memecah monolith mereka jadi microservices. Alasannya masuk akal di atas kertas: scaling independen, tim bisa kerja paralel tanpa saling ganggu, fault isolation. Semua benar, untuk konteks yang tepat.

Masalahnya, konteks kami bukan itu. Kami tim enam orang, dua di antaranya part-time. Tidak ada "tim" yang perlu kerja paralel tanpa saling ganggu — kami semua kerja di kode yang sama, tiap hari, saling ngobrol langsung kalau ada perubahan. Fault isolation yang kami dapat malah jadi bumerang: alih-alih satu proses crash dan gampang ditelusuri, kegagalan menyebar lewat network call antar service dengan error yang jauh lebih susah dilacak.

Saya nggak salah soal microservices sebagai pola. Saya salah nerapin pola yang didesain buat masalah organisasi besar ke tim yang masalah organisasinya belum ada.

## Titik baliknya

Yang bikin saya akhirnya berhenti bukan artikel counter-argument atau saran dari orang lain. Ini insiden spesifik: sistem notifikasi kami down selama empat jam karena message queue antara service order dan service notification penuh, dan tidak ada yang sadar sampai user komplain lewat email. Empat jam itu kami habiskan buka lima repo berbeda, cross-check log dari tiga service, sebelum ketemu akar masalahnya: queue consumer-nya crash diam-diam tanpa alert.

Kalau ini monolith, itu satu process, satu log file, dan kemungkinan besar ketauan dalam lima menit dari stack trace yang jelas.

Malam itu saya duduk, buka whiteboard virtual, dan mulai gambar ulang arsitektur dari nol. Bukan buat balik ke monolith yang naif — tapi buat jujur soal apa yang benar-benar kami butuhkan.

## Yang kami lakukan sekarang

Sembilan service jadi dua: satu monolith modular yang nanganin sebagian besar logic bisnis, dan satu service terpisah khusus buat processing yang memang butuh scaling independen (image processing, yang beban CPU-nya jauh beda dari trafik normal). Deploy sekarang satu langkah. Debugging satu log stream, dengan request ID yang konsisten dari ujung ke ujung.

Waktu deploy turun dari rata-rata 25 menit (termasuk koordinasi antar repo) ke di bawah 3 menit. Bukan cuma soal kecepatan — jumlah insiden production juga turun, karena permukaan kegagalan yang harus dipikirkan jauh lebih kecil.

Bukan berarti microservices salah secara umum. Kalau tim kami 40 orang dengan lima product line yang benar-benar independen, kemungkinan besar saya bakal balik mempertimbangkan pola serupa — dengan alasan yang lebih matang dari sekadar "perusahaan besar pakai ini".

## Yang saya bawa dari sini

Kalau ada satu hal yang saya pegang sekarang: jangan adopsi pola arsitektur karena itu yang dipakai perusahaan yang kalian kagumi. Adopsi karena masalah spesifik yang kalian punya, hari ini, benar-benar butuh pola itu.

Pertanyaan yang lebih berguna dari "arsitektur apa yang paling scalable" adalah "masalah organisasi atau teknis konkret apa yang bikin cara kerja kami sekarang nggak jalan". Kalau jawabannya nggak ada — atau jawabannya cuma "biar keliatan lebih profesional" — itu sinyal buat berhenti dulu.

Saya masih nyimpen diagram sembilan-service yang lama. Bukan buat nostalgia, tapi buat pengingat: kompleksitas itu gampang ditambah, susah dicabut lagi.
