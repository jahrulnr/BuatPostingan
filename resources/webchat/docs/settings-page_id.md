# Halaman Settings

Panel konfigurasi dengan tiga section: General, Users, dan Models (provider LLM). Product knobs tersimpan di `storage/config.json`; API key dan GitHub token di-mask saat dibaca.

## Overview

Halaman Settings adalah tampilan full-page yang menggantikan workspace chat saat aktif. Diaktifkan via URL hash route: `#/settings/general`, `#/settings/users`, `#/settings/models`, dan `#/settings/models/<providerId>`. Halaman punya navigation bar kiri dan panel konten kanan yang di-scroll biasa (tanpa floating sticky bar). Tombol back di atas nav kembali ke chat.

## Informasi yang ditampilkan ke pengguna

- **Navigation bar** (sisi kiri, tinggi penuh):
  - **Area atas**: Tombol back (ikon panah kiri, tombol abu-abu kecil) di kiri, teks bold "Settings" di sebelahnya.
  - **Nav list** (di bawah area atas): Tiga item link, masing-masing dengan ikon + label:
    - General (ikon sliders) → `#/settings/general`
    - Users (ikon people) → `#/settings/users`
    - Models (ikon CPU) → `#/settings/models`
    - Link section aktif disorot dengan aksen teal di tepi kiri.
  - **Area bawah**: Tombol Logout (ikon box-arrow-right, tombol abu-abu full-width).
- **Panel konten** (sisi kanan): Merender konten berbeda berdasarkan section aktif. Menampilkan "Loading…" saat mengambil data.
- **Settings toast** (bawah panel konten): Tersembunyi default. Menampilkan pesan sukses/error singkat selama ~3 detik.

### Konten section General (`#/settings/general`)

Editor compact untuk product knobs dari `storage/config.json`.

- **Header**: Heading "General" dengan lede tentang `storage/config.json`. Baris meta opsional: sumber config (`file` / `env` / `mock`), path config, dan badge `stub` jika belum ada provider yang usable.
- **Jump link section** (di bawah header, ikut scroll halaman): `limits`, `llm`, `context`, `docs`, `search`, `mcp`. Klik untuk scroll ke section tersebut.
- **Kartu Limits**: field numerik — Max tool rounds, Speak floor TTL (sec), Lock TTL (sec), Turn timeout (sec).
- **Kartu LLM globals**:
  - Strategy (tombol segmen): Failover / Round robin / Switch.
  - Active provider (tombol segmen): Auto plus setiap ID provider yang terkonfigurasi.
  - Toggle Stream responses.
  - Vision (tombol segmen): Auto / On / Off.
  - Effort (tombol segmen): Auto / None / Min / Low / Med / High / XHigh / Max.
  - Field numerik: Attempt budget, Retry base (ms), Retry max (ms), Jitter (0–1).
- **Kartu Context**: Toggle Compaction; Max input (tok), Reserve (tok), Recent turns, Summary max (chars).
- **Kartu Docs**: Top K, Min score, App ID, toggle Fuzzy match.
- **Kartu Web search**: Field password GitHub token. Placeholder `ghp_…` jika belum diset, atau "Leave blank to keep ••••…XXXX" jika sudah ada. Kosongkan untuk mempertahankan secret yang tersimpan.
- **Kartu MCP**:
  - Toggle MCP enabled; Connect timeout (sec); Call timeout (sec).
  - Tombol "Add server" (kanan-atas header MCP).
  - Baris server: switch enabled, ID server, meta transport/command, tombol "Edit" dan "Remove".
  - Body Edit yang terbuka: ID, Transport (stdio / SSE / HTTP), Command, Args (koma), URL, Env (`KEY=value` per baris), Allow tools / Deny tools (daftar koma), toggle Trusted, toggle Allow mutations.
- **Aksi footer** (akhir form, dalam alur dokumen): hint "Unsaved changes" saat dirty, "Reset" (muat ulang snapshot), "Save config" (primer).

### Konten section Users

- Header: "Users" dengan lede "Local JSON users — no auth yet." Di kanan ada tombol "Add user" (primer, ikon plus).
- Tabel: Kolom — ID (code style), Name, Role, dan tombol aksi. Setiap baris punya Edit (ghost) dan Delete (danger). Empty state: "No users".

### Konten section Models (`#/settings/models`)

Grid katalog provider — satu kartu per keluarga provider yang dikenal, ditimpa state koneksi yang sudah dikonfigurasi bila ada.

- **Header**: Heading "Providers" dengan lede "Manage direct APIs and local AI gateways. Credentials stay masked." Meta opsional sumber/path config. Di kanan: tombol "Custom provider" (primer, ikon plus).
- **Grid provider**: Kartu katalog untuk keluarga seperti OpenRouter, OmniRoute, 9Router, OpenAI, Claude API. Setiap kartu menampilkan:
  - **Baris atas**: Ikon aksen + nama keluarga + auth type · dialek API; toggle enabled jika sudah dikonfigurasi.
  - **Deskripsi**: Blurb singkat keluarga (atau base URL untuk kartu instance custom).
  - **Baris koneksi**: Status — Not configured / Configured / Needs API key / Disabled — plus ID instance dan jumlah model chat jika sudah dikonfigurasi.
  - **Aksi**: "Configure" jika belum disetup; "Details" + "Delete" jika sudah dikonfigurasi.
  - Endpoint OpenAI-compatible custom **bukan** kartu katalog — pakai "+ Custom provider". Setiap custom connection tersimpan muncul sebagai kartu instance sendiri (nama, base URL, toggle enabled, status, Details, Delete).
  - Kartu instance ekstra juga muncul untuk provider terkonfigurasi yang tipenya tidak diklaim keluarga katalog singleton.
- Fallback empty copy menyebut env sampai first save (biasanya katalog selalu dirender).

### Detail provider (`#/settings/models/<id>`)

- Header: Link back "← Models", nama provider, dan lede ID · dialek API.
- **Kartu info**: Base URL (code), API key (masked atau —), Enabled (yes/no). Aksi: "Edit", "Import models". Kegagalan import menampilkan alert merah di bawah aksi plus toast error.
- **Kartu models**: "Available models" dengan tombol "Add". Setiap baris model menampilkan ID (code), label opsional, "Remove", badge kapabilitas (context window, max output, input modes, effort levels, tools), deskripsi opsional. Empty state: "No models".

## Apa yang bisa dilakukan di halaman ini

- Mengedit dan menyimpan product knobs (limits, LLM globals, context, docs, token web search, server MCP) dari General.
- Menambah, mengedit, dan menghapus user lokal (dialog aplikasi — bukan prompt browser).
- Mengonfigurasi provider katalog, menambah provider OpenAI-compatible custom, enable/disable, edit, hapus.
- Menambah, menghapus, dan mengimpor model di halaman detail provider.
- Menghapus preferensi lokal via Logout dan kembali ke chat.

## Cara menggunakan fitur-fitur halaman ini

### Membuka Settings

1. Klik tombol ikon gear (pojok kanan bawah section profil rail percakapan).
2. Halaman Settings terbuka ke Models secara default (URL `#/settings/models`).

### Berpindah antar section

1. Klik General, Users, atau Models di nav kiri.
2. Panel konten diperbarui; link aktif disorot.

### Mengedit config General

1. Buka General (`#/settings/general`).
2. Ubah field di section mana pun (Limits, LLM globals, Context, Docs, Web search, MCP). Tombol segmen dan toggle menandai form dirty ("Unsaved changes").
3. Opsional: klik jump link (`limits`, `llm`, …) untuk scroll ke section itu.
4. Klik "Save config" di bawah. Toast mengonfirmasi "Config saved" dan form dimuat ulang dari server.
5. Klik "Reset" untuk membuang edit belum tersimpan dan memuat ulang snapshot saat ini.
6. Untuk GitHub token: ketik nilai baru untuk mengganti; kosongkan untuk mempertahankan secret tersimpan.
7. Untuk MCP: klik "Add server", isi field lewat "Edit", atau "Remove" baris. Save menulis daftar servers penuh.

### Menambah user

1. Buka Users.
2. Klik "Add user".
3. Dialog aplikasi meminta Name dan Role (tombol pilihan Owner / Admin / Member). Konfirmasi.
4. Toast: "User created".

### Mengedit atau menghapus user

1. Klik Edit atau Delete pada baris tabel.
2. Edit membuka dialog aplikasi; Delete membuka dialog konfirmasi aplikasi (tone danger).
3. Toast mengonfirmasi hasil. Owner terakhir tidak bisa dihapus atau diturunkan.

### Mengonfigurasi provider katalog

1. Buka Models.
2. Pada kartu "Not configured", klik "Configure".
3. Dialog provider terbuka dengan default keluarga (type, name, prefix, API, base URL). Isi API key dan model id opsional; sesuaikan field bila perlu.
4. Klik Save. Status kartu menjadi Configured (atau Needs API key jika key wajib dan belum diisi).

### Menambah provider custom

1. Di Models, klik "Custom provider" (endpoint OpenAI-compatible — tidak ada kartu katalog terpisah untuk keluarga ini).
2. Isi dialog (name, ID, base URL, API key, model id, …). Save membuat kartu koneksi standalone.

### Mengedit provider / mengelola model

1. Klik "Details" pada kartu yang sudah dikonfigurasi → `#/settings/models/<id>`.
2. Klik "Edit" untuk update field; kosongkan API key untuk mempertahankan secret yang ada.
3. Klik "Add" di Available models — dialog aplikasi untuk model id dan label opsional.
4. Klik "Remove" pada baris model untuk menghapusnya.
5. Klik "Import models" untuk menarik dari endpoint `/models` provider. Sukses menampilkan toast dengan jumlah imported/updated. Gagal menampilkan alert merah inline di bawah aksi (`code · message`, mis. `upstream · fetch models`) plus toast error yang sama — tombol kembali ke "Import models" agar bisa diulang.

### Enable/disable provider

1. Gunakan toggle enabled pada kartu Models yang sudah dikonfigurasi.
2. Toast menampilkan "Enabled" atau "Disabled". Jika gagal, toggle kembali.

### Menghapus provider

1. Klik "Delete" pada kartu yang sudah dikonfigurasi.
2. Konfirmasi di dialog aplikasi. Toast: "Provider deleted".

### Kembali ke chat

1. Klik panah back di atas nav settings, atau klik Logout (menghapus preferensi lokal: theme, model, effort, mock mode, preview width) lalu kembali ke chat.
2. URL berubah ke `#/`.
