# Toolbar Composer

Area input bawah dengan model picker, workspace picker, lampiran, dan kontrol kirim.

## Overview

Toolbar composer adalah section bawah dari kartu chat. Ini adalah titik interaksi utama untuk mengirim pesan ke agen AI. Composer punya dua baris:

1. **Baris toolbar** (atas): Pill model picker (kiri) dan pill workspace picker (kanan).
2. **Baris input** (bawah): Tombol lampir (kiri), input teks (tengah), tombol kirim/stop (kanan).

Di atas kedua baris ada area chip lampiran kondisional.

## Informasi yang ditampilkan ke pengguna

- **Chip lampiran** (di atas baris toolbar): Tersembunyi default. Muncul hanya saat ada file pending. Setiap chip menampilkan: thumbnail preview (gambar) atau ikon dokumen-file (teks) di kiri, nama file dan ukuran format (mis. "1.2 KB") di tengah, dan tombol hapus (X) di kanan.
- **Pill model picker** (sisi kiri baris toolbar): Tombol dengan ikon CPU di kiri, teks label di tengah (mis. "gpt-4o-mini · auto"), dan ikon chevron-down di kanan. Tooltip: "Model & effort".
- **Pill workspace picker** (sisi kanan baris toolbar): Tombol dengan ikon folder-open di kiri, teks label di tengah (mis. "my-project" atau "Workspace"), dan ikon chevron-down di kanan. Tooltip: "Workspace folder".
- **Tombol lampir** (sisi kiri baris input): Ikon paperclip. Tooltip: "Attach file". Abu-abu saat AI sibuk atau floor diblokir.
- **Input teks** (tengah baris input): Field teks dengan placeholder "Ketik pertanyaan…". Abu-abu saat AI sibuk atau floor diblokir.
- **Tombol kirim** (sisi kanan baris input): Ikon pesawat kertas saja. Abu-abu saat input kosong dan tidak ada lampiran pending. Disembunyikan saat AI merespons (diganti Stop di slot yang sama).
- **Tombol Stop** (sisi kanan baris input, slot sama dengan Kirim): Ikon kotak stop saja (tanpa label). Tersembunyi default. Muncul hanya saat AI merespons. Abu-abu untuk non-inisiator.

### Dropdown model picker

Tersembunyi default. Terbuka saat pill model picker diklik. Berisi:

- **Field pencarian** (atas dropdown): Input teks dengan ikon kaca pembesar di kiri, placeholder "Search models…". Auto-focus saat dibuka.
- **Daftar model** (di bawah search): Setiap baris model menampilkan:
  - Tombol model dengan nama model (kiri) dan tag metadata (kanan, mis. "vision · reasoning"). Model terpilih disorot.
  - Jika model mendukung reasoning effort: baris tombol effort di bawah entri model, menampilkan opsi seperti "auto", "none", "minimal", "low", "medium", "high", "xhigh", "max". Effort terpilih disorot.
- Empty state: "No models match "query"" atau "Models unavailable" (saat error).

### Dialog workspace picker

Tersembunyi default. Terbuka saat pill workspace picker diklik. Lihat detail di bawah.

## Apa yang bisa dilakukan di halaman ini

- Memilih model LLM dan effort level.
- Memilih folder workspace untuk operasi file.
- Melampirkan file (teks atau gambar) ke pesan.
- Mengetik dan mengirim pesan.
- Menghentikan turn AI yang sedang berjalan.

## Cara menggunakan fitur-fitur halaman ini

### Memilih model

1. Klik pill model picker (sisi kiri baris toolbar, ikon CPU).
2. Dropdown terbuka di bawah pill. Field pencarian (ikon kaca pembesar) berada di atas — auto-focus.
3. Ketik di field pencarian untuk memfilter model berdasarkan nama, ID, atau provider. Daftar diperbarui secara real-time.
4. Klik tombol model untuk memilih. Dropdown tertutup dan label pill diperbarui.
5. Jika model mendukung reasoning effort, tombol effort level muncul di bawah entri model di dropdown. Klik tombol effort (mis. "auto", "low", "high") untuk mengatur — dropdown tertutup.
6. Pilihan bertahan antar sesi.
7. Tekan Escape atau klik di luar dropdown untuk menutup tanpa memilih.

### Memilih folder workspace

1. Klik pill workspace picker (sisi kanan baris toolbar, ikon folder-open).
2. Dialog workspace muncul di tengah dengan backdrop gelap. Header dialog menampilkan ikon folder-open, label "Workspace", judul "Pilih folder workspace", dan tombol close (X) di pojok kanan atas.
3. Body dialog adalah folder browser:
   - **Path bar** (atas): Tombol Up (ikon panah-atas, kiri) dan teks path saat ini (mis. "/") di sebelahnya. Tombol Up abu-abu saat di root.
   - **Daftar direktori** (di bawah path bar): Daftar subdirektori yang dapat di-scroll. Setiap entri adalah tombol dengan ikon folder dan nama folder. Klik untuk navigasi masuk. Direktori terpilih disorot.
   - **Teks bantuan** di bawah daftar: "Pilih folder untuk mengatur working directory. Path absolut digunakan tanpa restriction."
   - **Teks error** (merah, tersembunyi default): Menampilkan error browse.
4. Klik "Select folder" (tombol biru, kanan bawah dialog, ikon check) untuk menetapkan direktori saat ini sebagai workspace.
5. Klik "Clear (use default)" (tombol ghost abu-abu, kiri) untuk reset ke workspace default config.
6. Klik "Cancel" (tombol ghost abu-abu, tengah) atau backdrop atau ikon X untuk membatalkan.
7. Workspace bertahan per percakapan antar sesi.

### Melampirkan file

1. Klik tombol paperclip (sisi kiri baris input).
2. Dialog lampiran muncul di tengah dengan backdrop. Header dialog menampilkan ikon paperclip, label "Attachments", judul "Lampirkan file", dan tombol close (X).
3. Body dialog adalah drop zone dengan ikon cloud-upload dan teks "Drop file di sini". Tarik file ke drop zone atau klik tombol "Pilih file" (biru, ikon folder-open) untuk membuka file picker OS.
4. Tipe yang diterima: teks (md, txt, json, csv, xml, yaml, html) dan gambar (png, jpg, gif, webp). Maks 8 MB per file — file kebesaran menampilkan toast warning.
5. File terpilih muncul sebagai chip di atas toolbar composer. Chip gambar menampilkan thumbnail preview; chip teks menampilkan ikon dokumen-file. Setiap chip punya tombol hapus (X) di kanannya — klik untuk hapus.
6. Klik "Selesai" (tombol ghost abu-abu, kanan bawah dialog) untuk menutup dialog.
7. File di-upload saat Anda mengirim pesan.

### Mengirim pesan

1. Ketik teks di field input (tengah baris input, placeholder "Ketik pertanyaan…").
2. Tombol kirim (ikon pesawat kertas, paling kanan) menjadi aktif (tidak lagi abu-abu) saat ada teks atau lampiran pending.
3. Tekan Enter atau klik tombol kirim. Jika ada lampiran pending, di-upload dulu, lalu pesan dikirim dengan attachment ID.
4. Tombol kirim dan input abu-abu saat AI sedang merespons.

### Menghentikan turn AI

1. Saat AI sedang merespons, tombol kirim disembunyikan dan tombol Stop (ikon kotak stop saja) muncul di tempatnya — slot sama, paling kanan baris input.
2. Klik Stop untuk menghentikan. Tombol Stop hanya bisa diklik jika Anda adalah inisiator turn. Non-inisiator melihatnya abu-abu.
3. Setelah berhenti, tombol kirim muncul kembali dan input di-enable kembali.
