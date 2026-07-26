# Halaman Chat

Antarmuka percakapan utama tempat pengguna berinteraksi dengan asisten AI.

## Overview

Halaman chat adalah layar utama BuatPostingan. Menempati kolom tengah layout, di antara rail percakapan (sidebar kiri) dan panel preview (sidebar kanan). Menampilkan percakapan antara pengguna dan agen AI, termasuk pesan pengguna, langkah reasoning agen, panggilan tool dengan hasilnya, dan balasan akhir agen. Respons dialirkan secara real-time.

Kartu chat berisi, dari atas ke bawah:

1. **Room header** — judul percakapan, label meta, dan tombol rename.
2. **Floor banner** — peringatan kondisional saat admin lain menahan floor.
3. **Docs index banner** — peringatan kondisional saat docs index belum siap.
4. **Toast** — strip notifikasi singkat.
5. **Area pesan** — log bubble percakapan yang dapat di-scroll.
6. **Tombol New activity** — kondisional, muncul saat scroll ke atas.
7. **Status bar** — indikator status satu baris.
8. **Composer** — chip lampiran, toolbar (pill model + workspace), baris input.

## Informasi yang ditampilkan ke pengguna

- **Room header** (atas kartu): Teks judul percakapan (rata kiri) dan label sumber judul di bawahnya ("Auto title", "Manual title", atau "Naming…"). Di sisi kanan header ada tombol ikon pensil — selalu terlihat.
- **Pesan** (area scroll tengah): Pesan pengguna (rata kanan) dengan nama pengirim dan akhiran "· you". Pesan agen (rata kiri) dengan badge model opsional. Bubble reasoning dirender sebagai section yang dapat diciutkan dengan ikon chevron ke kanan dan judul "Thinking" — klik untuk expand/collapse. Bubble tool call dirender sebagai section yang dapat diciutkan dengan judul "Tool Calls" — menampilkan nama tool, argumen, ringkasan hasil, dan badge model. Bubble error dengan ikon segitiga peringatan merah, judul "Failed", detail error, trace ID opsional, dan tombol Retry dengan ikon panah melingkar.
- **Status bar** (bawah kartu di atas composer): Satu baris teks dengan warna state — "Ready" (hijau), "Streaming…" (amber), "Thinking…" (amber), "Indexing docs…" (amber), "AI locked · docs index" (merah), "Failed · bisa Retry" (merah).
- **Floor banner** (antara room header dan pesan): Tersembunyi secara default. Muncul hanya saat admin lain menahan speak-floor. Menampilkan ikon mic-mute dan "Admin #ID menahan floor · sisa Xm XXs" dengan hitung mundur langsung.
- **Docs index banner** (antara floor banner dan pesan): Tersembunyi secara default. Muncul hanya saat docs index belum siap. Menampilkan ikon hourglass dan "Docs index belum siap. AI terkunci."
- **Toast** (strip tipis di atas area pesan): Tersembunyi secara default. Meluncur masuk selama ~3 detik dengan pesan seperti "Docs index siap · AI aktif", "Conversation deleted", "Stop · floor tidak dilepas".
- **Tombol New activity** (melayang di atas status bar): Tersembunyi secara default. Muncul sebagai tombol pill di bawah-tengah area pesan saat pesan baru tiba dan pengguna sedang scroll ke atas. Menampilkan "New activity ↓". Klik untuk scroll ke terbaru.
- **Lampiran dalam pesan**: File gambar menampilkan thumbnail preview. File teks menampilkan ikon dokumen-file. Setiap chip lampiran menampilkan nama file dan ukuran file.
- **State welcome** (saat tidak ada percakapan dimuat): Kartu terpusat dengan ikon chat-bubble, heading "Selamat datang di BuatPostingan"
- **Indikator typing** (saat agen berpikir tapi belum ada teks): Tiga titik memantul dengan label "Thinking…" di dalam bubble agen.

## Apa yang bisa dilakukan di halaman ini

- Mengirim pesan teks ke agen AI.
- Melampirkan file (teks atau gambar) sebelum mengirim.
- Melihat langkah reasoning AI, tool call, dan teks yang dialirkan secara real-time.
- Menghentikan turn yang sedang berjalan (jika Anda adalah inisiator).
- Mencoba ulang turn yang gagal.
- Mengganti nama percakapan.
- Menghapus percakapan.
- Berpindah antar percakapan (melalui rail).
- Memilih model LLM dan level effort (melalui model picker di composer).
- Memilih folder workspace (melalui workspace picker).
- Membuka panel preview.
- Membuka pengaturan.

## Cara menggunakan fitur-fitur halaman ini

### Mengirim pesan

1. Temukan field input teks di bawah kartu (placeholder "Ketik pertanyaan…") — berada di antara tombol paperclip (kiri) dan tombol kirim (kanan).
2. Ketik pesan. Tombol kirim (ikon pesawat kertas, paling kanan baris input) menjadi aktif saat ada teks atau lampiran pending.
3. Tekan Enter atau klik tombol kirim. Pesan Anda muncul langsung di sisi kanan. Respons AI mengalir sebagai bubble di sisi kiri.

### Menghentikan turn

1. Saat AI sedang merespons, tombol kirim disembunyikan dan tombol Stop (ikon kotak stop saja) muncul di tempatnya — slot sama, paling kanan baris input.
2. Klik Stop untuk menghentikan. Tombol Stop hanya bisa diklik jika Anda adalah inisiator turn. Non-inisiator melihatnya abu-abu.
3. Setelah berhenti, status menampilkan "Interrupted · floor tetap Anda" dan toast muncul.

### Mencoba ulang turn yang gagal

1. Saat turn gagal, bubble error muncul di area pesan (rata kiri, merah, ikon segitiga peringatan).
2. Di bawah bubble error ada tombol Retry (ikon panah melingkar + teks "Retry").
3. Klik Retry untuk mengirim ulang pesan yang sama. Bubble error dihapus dan turn baru dimulai.

### Mengganti nama percakapan

1. Klik tombol ikon pensil (pojok kanan atas room header).
2. Dialog rename muncul di tengah layar dengan overlay backdrop gelap. Header dialog menampilkan ikon pencil-square, label "Conversation", judul "Rename conversation", dan tombol close (X) di pojok kanan atas.
3. Masukkan judul baru di input teks (maks 60 karakter). Penghitung karakter menampilkan "N/60" di sebelah kanan input.
4. Klik tombol "Save title" (biru, kanan bawah dialog, dengan ikon panah kanan) atau tekan Enter untuk submit.
5. Klik "Cancel" (tombol ghost abu-abu, kiri) atau klik backdrop atau ikon X untuk membatalkan.

### Menghapus percakapan

1. Di rail percakapan (sidebar kiri), arahkan kursor ke item percakapan untuk menampilkan tombol ikon tempat sampah di sisi kanannya.
2. Klik tombol tempat sampah. Dialog hapus muncul di tengah dengan backdrop. Header dialog menampilkan ikon tempat sampah, label "Conversation", judul "Delete conversation", dan tombol close (X).
3. Body dialog menampilkan pesan peringatan dengan nama percakapan. Klik "Delete" (tombol merah danger, kanan, dengan ikon tempat sampah) untuk konfirmasi, atau "Cancel" (ghost abu-abu, kiri) untuk membatalkan.

### Melampirkan file

1. Klik tombol paperclip (sisi kiri baris input composer).
2. Dialog lampiran muncul di tengah dengan backdrop. Header dialog menampilkan ikon paperclip, label "Attachments", judul "Lampirkan file", dan tombol close (X).
3. Body dialog adalah drop zone dengan ikon cloud-upload dan teks "Drop file di sini". Tarik file ke drop zone atau klik tombol "Pilih file" (biru, dengan ikon folder-open) untuk membuka file picker.
4. Tipe yang diterima: teks (md, txt, json, csv, xml, yaml, html) dan gambar (png, jpg, gif, webp). Maks 8 MB per file — file kebesaran menampilkan toast warning.
5. File terpilih muncul sebagai chip di atas input composer. Chip gambar menampilkan thumbnail preview; chip teks menampilkan ikon dokumen-file. Setiap chip punya tombol hapus (X) di kanannya.
6. Klik "Selesai" (tombol ghost abu-abu, kanan bawah dialog) untuk menutup dialog.
7. File di-upload saat Anda mengirim pesan.

### Berpindah percakapan

1. Klik item percakapan mana pun di rail kiri. Percakapan aktif disorot dengan warna background berbeda.
2. Area pesan dibersihkan dan memuat pesan percakapan terpilih.

### Expand/collapse langkah reasoning

1. Reasoning agen muncul sebagai section yang dapat diciutkan dengan ikon chevron ke kanan dan label "Thinking".
2. Klik header section untuk expand atau collapse langkah reasoning. Saat expanded, daftar bernomor langkah berpikir ditampilkan.

### Expand/collapse tool calls

1. Tool call muncul sebagai section yang dapat diciutkan dengan ikon chevron ke kanan dan label "Tool Calls", menampilkan jumlah (mis. "2 tool calls").
2. Klik header section untuk expand atau collapse. Saat expanded, setiap baris tool menampilkan signature panggilan tool (mis. `docs_search("query")`), ringkasan hasil, dan badge model jika tersedia.
