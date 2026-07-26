import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const copySources = [
    new URL('../../login.html', import.meta.url),
    new URL('../app.js', import.meta.url),
    new URL('./chat.js', import.meta.url),
    new URL('./model-picker.js', import.meta.url),
    new URL('./page-browser.js', import.meta.url),
    new URL('./render.js', import.meta.url),
    new URL('../api/mock/fixtures.js', import.meta.url),
    new URL('../api/mock/store.js', import.meta.url),
];

test('frontend copy uses English rather than the retired Indonesian UI wording', async function () {
    const contents = await Promise.all(copySources.map(function (url) {
        return readFile(url, 'utf8');
    }));
    const retiredCopy = /Belum ada|Klik kanan|belum tersedia|Memuat pages|Tidak bisa memuat|Aksi Pages|Menghapus page|Memperbarui status|Aksi page gagal|tidak tersedia|memakai default|Hapus |tetap Anda|tidak dilepas|sedang dibangun|sementara tidak tersedia|Kirim pesan dulu|Selamat datang|Ajukan pertanyaan|Username atau password|Membaca lampiran|Menyusun jawaban|Mencari referensi|Ini gambar|Vision belum aktif|Berikut ringkasan|Tentukan sudut|Susun outline|Tulis draft|Sumber:|File terlalu besar|untuk mock|Docs index belum siap|AI terkunci|index siap|AI aktif|sisa /;

    contents.forEach(function (source) {
        assert.doesNotMatch(source, retiredCopy);
    });
});
