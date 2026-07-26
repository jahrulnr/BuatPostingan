# Halaman Settings

Panel konfigurasi dengan tiga section: General, Users, dan Models (provider LLM).

## Overview

Halaman Settings adalah tampilan full-page yang menggantikan workspace chat saat aktif. Diaktifkan via URL hash route: `#/settings/general`, `#/settings/users`, `#/settings/models`. Halaman punya navigation bar kiri dan panel konten kanan. Tombol back di atas nav kembali ke chat.

## Informasi yang ditampilkan ke pengguna

- **Navigation bar** (sisi kiri, tinggi penuh):
  - **Area atas**: Tombol back (ikon panah kiri, tombol abu-abu kecil) di kiri, teks bold "Settings" di sebelahnya.
  - **Nav list** (di bawah area atas): Tiga item link, masing-masing dengan ikon + label:
    - General (ikon sliders) → `#/settings/general`
    - Users (ikon people) → `#/settings/users`
    - Models (ikon CPU) → `#/settings/models`
    - Link section aktif disorot.
  - **Area bawah**: Tombol Logout (ikon box-arrow-right, tombol abu-abu full-width).
- **Panel konten** (sisi kanan): Merender konten berbeda berdasarkan section aktif. Menampilkan "Loading…" saat mengambil data.
- **Settings toast** (bawah panel konten): Tersembunyi default. Menampilkan pesan sukses/error singkat selama ~3 detik.

### Konten section General

- Header: Heading "General" dengan lede "Theme and local preferences. Server env globals stay in `BP_*`."
- Kartu: Teks muted "Use the theme control in the top chrome. Logout (left nav) clears local prefs only."

### Konten section Users

- Header: Heading "Users" Di kanan ada tombol "Add user" (biru, ikon plus).
- Tabel: Kolom — ID (code style), Name, Role, dan tombol aksi. Setiap baris punya Edit (tombol ghost abu-abu) dan Delete (tombol merah danger) di kanan. Empty state menampilkan "No users" di baris muted.

### Konten section Models

- Header: Heading "Models" dengan lede "OpenAI-compatible LLM providers." Di kanan ada tombol "Add OpenAI Compatible" (biru, ikon plus). Di bawah header, baris meta opsional menampilkan sumber config dan path.
- **Grid provider**: Setiap provider adalah kartu berisi:
  - **Baris atas**: Nama provider (heading) dan teks ID/api type di kiri; toggle switch enabled di kanan. Provider disabled menampilkan badge "disabled".
  - **Base URL**: Teks code menampilkan base URL provider.
  - **API key**: "Key sk-…XXXX" (masked) atau "No API key" (muted).
  - **Jumlah model**: "Models: N" dengan badge jumlah.
  - **Baris aksi**: Tombol "Details" (ghost abu-abu) dan "Delete" (merah danger) di bawah kartu.
  - Empty state: "No providers yet — add one or rely on `BP_LLM_*` env until first save."

### Detail provider (`#/settings/models/<id>`)

- Header: Link back "← Models" (kiri-atas), nama provider (heading), dan lede ID + api type di bawah.
- **Kartu info**: Definition list menampilkan Base URL (code), API key (masked atau —), dan Enabled (yes/no). Tombol aksi: "Edit" (ghost abu-abu) dan "Import models" (ghost abu-abu) di bawah list.
- **Kartu models**: Sub-header "Available models" (heading) dengan tombol "Add" (biru, kecil, ikon plus) di kanan. Di bawah adalah daftar model. Setiap baris model menampilkan:
  - Model ID (code style) dan label opsional (muted).
  - Tombol "Remove" (ghost abu-abu, kecil, kanan baris).
  - Badge metadata di bawah: context window (mis. "128K ctx"), max output (mis. "16K out"), input modes (mis. "text", "image"), effort levels (mis. "low", "high"), dan badge "tools" jika mendukung tool.
  - Deskripsi opsional di bawah badge.
  - Empty state: "No models" (muted).

## Apa yang bisa dilakukan di halaman ini

- Melihat info tema dan menghapus preferensi lokal (Logout).
- Menambah, mengedit, dan menghapus user lokal.
- Menambah, mengedit, menghapus, enable/disable provider LLM.
- Menambah dan menghapus model individual dari provider.
- Import model dari API provider.
- Kembali ke halaman chat.

## Cara menggunakan fitur-fitur halaman ini

### Membuka Settings

1. Klik tombol ikon gear (pojok kanan bawah section profil rail percakapan).
2. Halaman Settings terbuka. Section Models ditampilkan secara default (URL `#/settings/models`).

### Berpindah antar section

1. Klik link mana pun di nav list kiri: General (ikon sliders), Users (ikon people), atau Models (ikon CPU).
2. Panel konten di kanan diperbarui. Link aktif disorot.

### Menambah user

1. Buka section Users (klik "Users" di nav kiri, ikon people).
2. Klik tombol "Add user" (biru, kanan-atas header users, ikon plus).
3. Browser prompt muncul menanyakan "Name". Masukkan nama dan klik OK.
4. Prompt kedua menanyakan "Role (owner|admin|member)" dengan default "member". Masukkan role dan klik OK.
5. User muncul di tabel. Toast mengonfirmasi "User created".

### Mengedit user

1. Di tabel Users, klik tombol "Edit" (tombol ghost abu-abu, kanan baris).
2. Browser prompt muncul dengan nama saat ini. Masukkan nama baru (kosongkan untuk tetap) dan klik OK.
3. Prompt kedua menanyakan role. Masukkan role baru (kosongkan untuk tetap) dan klik OK.
4. Perubahan disimpan. Toast mengonfirmasi "User updated".

### Menghapus user

1. Di tabel Users, klik tombol "Delete" (merah danger, kanan baris, di sebelah Edit).
2. Browser confirmation dialog muncul: "Delete user ID?". Klik OK untuk konfirmasi.
3. User dihapus. Toast mengonfirmasi "User deleted".

### Menambah provider LLM

1. Buka section Models (klik "Models" di nav kiri, ikon CPU).
2. Klik tombol "Add OpenAI Compatible" (biru, kanan-atas header models, ikon plus).
3. Dialog provider muncul di tengah dengan backdrop gelap. Header dialog menampilkan ikon plug, label "LLM", judul "Add Provider", dan tombol close (X) di pojok kanan atas.
4. Isi field form:
   - **Name** (input full-width, placeholder "OpenRouter") — required.
   - **ID / prefix** (input full-width, placeholder "OPENROUTER") — required.
   - **Prefix (optional)** (input half-width, placeholder "openrouter").
   - **API type** (dropdown half-width: "responses" atau "chat").
   - **Base URL** (input full-width, placeholder "https://openrouter.ai/api/v1") — required.
   - **API key** (input password full-width, placeholder "sk-…").
   - **Model id** (input full-width, placeholder "openai/gpt-4o-mini").
   - **Enabled** (checkbox, tercentang default).
5. Klik "Save" (tombol biru, kanan bawah dialog) untuk membuat. Klik "Cancel" (ghost abu-abu, kiri) atau backdrop untuk membatalkan.

### Mengedit provider

1. Klik tombol "Details" (ghost abu-abu, bawah-kiri kartu provider).
2. Detail view provider terbuka. URL berubah ke `#/settings/models/<id>`.
3. Klik tombol "Edit" (ghost abu-abu, di bawah kartu info, sisi kiri).
4. Dialog provider yang sama muncul, sudah terisi nilai provider saat ini. Field ID read-only (abu-abu).
5. Field API key menampilkan placeholder "Leave blank to keep sk-…XXXX" — kosongkan untuk mempertahankan key yang ada.
6. Ubah field yang diperlukan. Klik "Save" untuk update.

### Menghapus provider

1. Klik tombol "Delete" (merah danger, bawah-kanan kartu provider, di sebelah Details).
2. Browser confirmation dialog muncul: "Delete provider ID?". Klik OK untuk konfirmasi.
3. Kartu provider dihapus. Toast mengonfirmasi "Provider deleted".

### Enable/disable provider

1. Temukan toggle switch enabled di pojok kanan-atas kartu provider.
2. Klik toggle untuk membalik. Perubahan langsung disimpan.
3. Toast menampilkan "Enabled" atau "Disabled". Jika API call gagal, toggle kembali.

### Menambah model ke provider

1. Buka detail view provider (klik "Details" pada kartu provider).
2. Di section "Available models", klik tombol "Add" (biru, kecil, kanan-atas sub-header models, ikon plus).
3. Browser prompt menanyakan "Model id". Masukkan model ID dan klik OK.
4. Prompt kedua menanyakan "Label (optional)". Masukkan label atau kosongkan, klik OK.
5. Model muncul di daftar. Toast mengonfirmasi "Model added".

### Menghapus model

1. Di detail view provider, temukan model di daftar "Available models".
2. Klik tombol "Remove" (ghost abu-abu, kecil, kanan baris model).
3. Model langsung dihapus. Toast mengonfirmasi "Model removed".

### Import model dari API

1. Buka detail view provider (klik "Details" pada kartu provider).
2. Klik tombol "Import models" (ghost abu-abu, di bawah kartu info, kanan "Edit").
3. Sistem mengambil model tersedia dari API provider. Toast menampilkan pesan hasil.
4. Model yang diimpor muncul di daftar "Available models".

### Kembali ke chat

1. Klik tombol panah back (ikon panah kiri, kiri-atas nav settings).
2. Atau klik "Logout" (ikon box-arrow-right, bawah nav settings) untuk menghapus preferensi lokal dan kembali ke chat.
3. Workspace chat muncul kembali. URL berubah ke `#/`.
