# Panel Preview

Panel kanan dengan tab Preview dan Browser untuk output kerja AI.

## Overview

Panel preview adalah sidebar yang dapat diciutkan di sisi kanan workspace chat. Terlihat secara default di desktop dan tersembunyi di mobile (layar lebih sempit dari 1101px). Panel punya tab bar di atas dan area body di bawah. Handle resize vertikal berada di antara area chat utama dan panel preview.

## Informasi yang ditampilkan ke pengguna

- **Tab bar** (atas panel): Dua tombol tab berdampingan:
  - **Tab Preview**: Aktif secara default (disorot). Teks "Preview".
  - **Tab Browser**: Tidak aktif default. Teks "Browser".
- **Body panel Preview** (di bawah tab bar, aktif default): Empty state terpusat di panel — ikon desktop/window, label bold "Kosong", dan deskripsi "Akan diisi nanti — wrapper preview kerja AI."
- **Body panel Browser** (di bawah tab bar, tersembunyi default): Empty state terpusat — ikon globe, label bold "Kosong", dan deskripsi "Akan diisi nanti — tidak ada mock content."
- **Handle resize** (bar vertikal antara area chat dan panel preview): Bar pemisah tipis dengan tooltip "Drag to resize · double-click to reset". Hanya terlihat di desktop (≥ 1101px).

## Apa yang bisa dilakukan di halaman ini

- Berpindah antara tab Preview dan Browser.
- Membuka/menutup seluruh panel preview.
- Mengubah lebar panel dengan menarik splitter (desktop saja).
- Mereset lebar panel dengan double-click splitter (desktop saja).

## Cara menggunakan fitur-fitur halaman ini

### Berpindah tab

1. Klik tab "Preview" (tab kiri di tab bar atas panel) atau tab "Browser" (tab kanan).
2. Tab yang diklik menjadi disorot sebagai aktif. Tab lain dinonaktifkan.
3. Panel body yang sesuai menjadi terlihat; yang lain disembunyikan.

### Membuka/menutup panel

1. Klik tombol toggle preview (ikon layout-sidebar-reverse, sisi kanan header bar atas).
2. Panel masuk/keluar.
3. Klik lagi untuk toggle kembali.

### Mengubah lebar panel (desktop saja, ≥ 1101px)

1. Temukan handle resize vertikal — bar tipis antara area chat (kiri) dan panel preview (kanan).
2. Klik dan tarik handle: tarik ke kiri untuk melebarkan panel preview, tarik ke kanan untuk mengecilkannya.
3. Lebar dibatasi: minimum 240px, maksimum adalah setengah lebar window atau ruang tersedia dikurangi rail dan minimum area chat (280px).
4. Lebar yang dipilih bertahan antar sesi.
5. Saat menarik, kursor berubah menunjukkan resizing.

### Mereset lebar panel

1. Double-click handle resize.
2. Lebar panel direset ke default 320px.

### Di mobile (< 1101px)

Panel preview, tab bar, dan handle resize semua tersembunyi. Panel tidak tersedia di layar mobile.
