# Rail Percakapan

Sidebar kiri yang menampilkan semua percakapan dan menyediakan navigasi antar percakapan.

## Overview

Rail percakapan adalah sidebar yang dapat diciutkan di sisi kiri workspace chat. Terlihat secara default di desktop dan tersembunyi di mobile (ditoggle via tombol toggle sidebar di header bar atas). Rail berisi tiga section vertikal: section atas (heading + new chat + search), daftar percakapan tengah yang dapat di-scroll, dan section profil bawah dengan ikon gear settings.

## Informasi yang ditampilkan ke pengguna

- **Heading workspace** (atas rail): Teks eyebrow kecil "Workspace" di atas label bold "Conversations". Di kanan ada badge jumlah (berbentuk pill) menampilkan total percakapan — menampilkan "—" saat loading.
- **Tombol New chat** (di bawah heading): Tombol full-width dengan ikon plus dan teks "New chat".
- **Field pencarian** (di bawah New chat): Input pencarian dengan ikon kaca pembesar di kiri, placeholder "Cari percakapan…".
- **Daftar percakapan** (tengah rail, dapat di-scroll): Setiap item menampilkan:
  - Teks judul (kiri, bold; italic jika judul auto masih pending).
  - Pill badge sumber di kanan judul — "Auto", "Renamed", "Stale", atau "Naming…" dengan background berwarna.
  - Baris meta di bawah dengan ID pembuat (mis. "#1") dan waktu relatif (mis. "just now", "2h", "3d").
  - Tombol ikon tempat sampah di sisi kanan — tersembunyi default, muncul saat hover.
  - Percakapan aktif disorot dengan warna background berbeda.
- **Section profil** (bawah rail): Avatar lingkaran orang (kiri), teks meta profil ("Owner" bold, "local" subtitle) di tengah, dan tombol ikon gear di kanan dengan tooltip "Settings".

## Apa yang bisa dilakukan di halaman ini

- Membuat percakapan baru.
- Mencari percakapan berdasarkan judul.
- Berpindah ke percakapan lain.
- Menghapus percakapan (melalui tombol aksi saat hover).
- Membuka Settings.
- Menciutkan/membuka rail (melalui tombol toggle di header atas).

## Cara menggunakan fitur-fitur halaman ini

### Membuat percakapan baru

1. Klik tombol "New chat" (atas rail, ikon plus).
2. Percakapan kosong baru dimulai. Kartu welcome muncul di area pesan. Percakapan sebelumnya tetap ada di daftar.

### Mencari percakapan

1. Klik field pencarian (di bawah tombol New chat, ikon kaca pembesar di kiri).
2. Ketik query. Daftar terfilter secara real-time berdasarkan judul yang cocok. Item tidak cocok langsung disembunyikan.

### Berpindah percakapan

1. Klik item percakapan mana pun di daftar. Item aktif disorot dengan warna background berbeda.
2. Area chat memuat pesan percakapan tersebut.

### Menghapus percakapan

1. Arahkan kursor ke item percakapan di daftar. Tombol ikon tempat sampah muncul di sisi kanan item.
2. Klik tombol tempat sampah. Dialog hapus muncul di tengah dengan overlay backdrop gelap. Header dialog menampilkan ikon tempat sampah, label "Conversation", judul "Delete conversation", dan tombol close (X) di pojok kanan atas.
3. Body dialog menampilkan peringatan: "Delete "name"? This conversation and all its messages will be permanently removed. This action cannot be undone."
4. Klik "Delete" (tombol merah danger, kanan, dengan ikon tempat sampah) untuk konfirmasi. Klik "Cancel" (tombol ghost abu-abu, kiri) atau backdrop untuk membatalkan.

### Menciutkan/membuka rail

1. Klik tombol toggle sidebar (ikon layout-sidebar, sisi kiri header bar atas).
2. Rail bergeser keluar. Klik lagi untuk membuka kembali.
3. Di mobile (layar lebih sempit dari 1101px), menapak overlay gelap di luar rail juga menutup rail.

### Membuka Settings

1. Klik tombol ikon gear (pojok kanan bawah section profil rail).
2. Halaman Settings terbuka, menggantikan workspace chat. URL berubah ke `#/settings/models`.
