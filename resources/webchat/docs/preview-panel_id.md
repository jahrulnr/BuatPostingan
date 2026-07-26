# Panel Preview

Panel kanan dengan tab Preview dan Pages untuk output kerja AI serta lifecycle page yang dioperasikan pengguna.

## Overview

Panel preview adalah sidebar yang dapat diciutkan di sisi kanan workspace chat. Terlihat secara default di desktop dan tersembunyi di mobile (layar lebih sempit dari 1101px). Panel punya tab bar di atas dan area body di bawah. Handle resize vertikal berada di antara area chat utama dan panel preview.

## Informasi yang ditampilkan ke pengguna

- **Tab bar** (atas panel): Dua tombol tab berdampingan:
  - **Tab Preview**: Aktif secara default (disorot). Teks "Preview".
  - **Tab Pages**: Tidak aktif default. Teks "Pages".
- **Body panel Preview** (di bawah tab bar, aktif default): Memuat draft page `home` di iframe bila page itu ada (Docker men-seed dan mem-publish `home` pada boot pertama). Jika `home` belum ada, menampilkan empty state — ikon desktop/window, label bold "Empty", dan catatan singkat bahwa panel default ke `home` bila tersedia. Setelah agen sukses menjalankan `page_create` / `page_edit` / `page_delete`, iframe beralih ke draft page tersebut.
- **Body panel Pages** (di bawah tab bar, tersembunyi default): Tree folder untuk setiap page workspace. Folder page dapat di-expand untuk melihat file dan folder relatif seperti `assets/`, `index.html`, `page.css`, dan `page.js`. Setiap folder page memiliki badge **Draft** atau **Published**.
- **Menu konteks Pages**: Klik kanan folder page membuka aksi Publish, Unpublish, dan Delete. Publish/Unpublish mengubah marker publikasi page. Delete menghapus seluruh workspace page dan meminta konfirmasi browser terlebih dahulu.
- **Handle resize** (bar vertikal antara area chat dan panel preview): Bar pemisah tipis dengan tooltip "Drag to resize · double-click to reset". Hanya terlihat di desktop (≥ 1101px).

## Apa yang bisa dilakukan di halaman ini

- Berpindah antara tab Preview dan Pages.
- Melihat tree page workspace dan status Draft/Published.
- Publish, unpublish, atau menghapus page secara manual melalui menu konteks.
- Membuka/menutup seluruh panel preview.
- Mengubah lebar panel dengan menarik splitter (desktop saja).
- Mereset lebar panel dengan double-click splitter (desktop saja).

## Cara menggunakan fitur-fitur halaman ini

### Berpindah tab

1. Klik tab "Preview" (tab kiri di tab bar atas panel) atau tab "Pages" (tab kanan).
2. Tab yang diklik menjadi disorot sebagai aktif. Tab lain dinonaktifkan.
3. Panel body yang sesuai menjadi terlihat; yang lain disembunyikan.

### Mengelola pages

1. Buka tab "Pages". Tree dimuat ulang saat tab dibuka; gunakan tombol refresh bila perlu.
2. Klik folder page untuk expand/collapse isi workspace.
3. Klik kanan folder page untuk membuka menu konteks.
4. Pilih **Publish** untuk membuat page live, atau **Unpublish** untuk menurunkannya tanpa mengubah draft.
5. Pilih **Delete** untuk menghapus page beserta seluruh filenya. Konfirmasi browser harus disetujui sebelum penghapusan dilakukan.

Agent dapat membuat, membaca, dan mengedit page melalui tool page authoring, tetapi tidak memiliki akses `page_delete`. Penghapusan hanya dilakukan pengguna dari tab Pages.

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
