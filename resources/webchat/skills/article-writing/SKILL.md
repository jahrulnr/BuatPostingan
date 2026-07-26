---
name: article-writing
description: Use this skill whenever writing long-form content — articles, blog posts, reports, technical explainers, essays, documentation prose — where the goal is professional-but-not-stiff writing that flexibly shifts between technical, formal, and casual register within the same piece. Trigger for requests like "write an article about X", "buatkan artikel", "explain this in blog form", or any long-form deliverable where the user wants it to read like it was written by a skilled human writer, not a generic AI-generated listicle. Different from natural-conversational-style, which is for chat/dialogue — this skill governs continuous prose structure, paragraph rhythm, and register-shifting within formal writing.
---

# Humanized Article Writing

Skill ini untuk nulisan panjang (artikel, blog, report, penjelasan teknis) yang
tetep profesional tapi enak dibaca — bukan kaku kayak white paper korporat,
juga bukan terlalu santai kayak obrolan. Beda sama skill `natural-conversational-style`
yang buat dialog/chat.

Ciri tulisan bagus: penulis tau **kapan** harus teknis (pas emang perlu presisi),
**kapan** boleh santai (pas ngasih analogi/pembuka), dan **kapan** harus formal
(pas nyatain klaim/kesimpulan) — semuanya dalam satu artikel yang sama, mengalir,
tanpa terasa nyambung-nyambungin gaya yang beda-beda.

---

## 1. Register-shifting dalam satu piece

Jangan pilih satu level formalitas terus dipakai rata dari awal sampai akhir.
Pola yang natural:
- **Hook/pembuka**: boleh lebih santai, konkret, atau pakai analogi — tarik perhatian
  dulu sebelum masuk substansi.
- **Badan penjelasan teknis**: presisi, istilah teknis dijaga akurat, tapi tetap
  dijelasin dengan kalimat manusia, bukan definisi kamus.
- **Transisi antar-section**: nada sedikit turun ke conversational, biar nggak
  berasa loncat dari satu textbook chapter ke chapter lain.
- **Kesimpulan**: balik agak formal untuk nyatain implikasi, tapi jangan cuma
  ngulang isi paragraf sebelumnya dengan kata lain.

Aturan praktis: kalau satu paragraf isinya klaim/data penting → formal & presisi.
Kalau paragraf isinya nyambungin ide atau kasih konteks → boleh lebih santai.

## 2. Pola "AI-tell" dalam prosa panjang (paling sering ketauan)

Ini yang paling gampang bikin tulisan "kebaca AI" — hindari:

| Pola | Kenapa masalah | Ganti dengan |
|---|---|---|
| "Not just X, but Y" berulang tiap beberapa paragraf | jadi tic/formula, bukan penekanan asli | variasikan struktur penekanan, atau langsung nyatain klaimnya |
| Rule-of-three dipaksa ("cepat, efisien, dan handal") tiap listing hal | kedengeran template, bukan observasi spesifik | sebutin 1-2 hal yang paling relevan, spesifik ke konteks |
| Pembuka generik ("Di era digital saat ini...", "Dalam dunia yang serba cepat...") | tidak menambah informasi, basi | mulai langsung dari fakta/kasus konkret |
| Kesimpulan yang cuma ngulang isi ("Sebagai kesimpulan, X penting karena...") | nggak nambah nilai, terasa wajib-formalitas | kesimpulan berisi implikasi baru atau pertanyaan lanjutan, bukan rekap |
| Overuse "furthermore/moreover/di sisi lain" sebagai jembatan tiap paragraf | transisi jadi mekanis | transisi lewat isi kalimat itu sendiri (referensi ke ide sebelumnya secara implisit) |
| Heading/bullet dipaksa untuk konten yang sebenernya lebih enak jadi paragraf mengalir | fragmentasi berlebihan, hilang nuansa | pakai heading/bullet cuma untuk struktur yang emang butuh scanning (steps, perbandingan), sisanya prosa |
| Qualifier/hedge ditumpuk ("bisa jadi", "mungkin", "cenderung") dalam satu kalimat | melemahkan klaim tanpa alasan jelas | pilih satu tingkat keyakinan, nyatain jelas |
| Em-dash atau kolon dipakai berlebihan buat "dramatic pause" tiap kalimat | jadi mannerism yang gampang dikenali | variasikan tanda baca, banyak kalimat cukup titik biasa |
| "Bukan X, tapi/melainkan Y" (antitesis) dipakai berulang sebagai formula penekanan tiap beberapa paragraf | ini versi Indonesia dari "not just X but Y" — sama-sama jadi tic kalau polanya diulang-ulang, meskipun secara literal beda kalimat | variasikan: kadang langsung nyatain Y tanpa nolak X dulu, kadang pakai sebab-akibat atau contoh konkret sebagai penekanan |

## 3. Variasi ritme kalimat

Tulisan manusia yang bagus punya variasi panjang kalimat: kalimat pendek buat
penekanan, kalimat panjang buat elaborasi/nuansa, diselang-seling. Kalau semua
kalimat panjangnya seragam (entah semua pendek atau semua kompleks), itu ciri
khas tulisan yang terasa di-generate, bukan ditulis.

Praktik: setelah kalimat panjang berisi klaim kompleks, ikuti dengan kalimat
pendek yang menegaskan poinnya. Contoh:

> "Penurunan latency ini bukan cuma soal infrastruktur — arsitektur agen yang
> menghindari round-trip berlebih ke API eksternal juga berkontribusi besar.
> Intinya: desain, bukan cuma server."

## 4. Spesifisitas > abstraksi

Klaim umum tanpa detail konkret adalah ciri paling gampang dikenali tulisan
generic. Selalu cek: apakah kalimat ini bisa diganti topiknya tanpa berubah
sama sekali (mis. bisa dipakai buat artikel apa aja)? Kalau iya, itu terlalu
generik — tambahin angka, nama, contoh spesifik, atau detail teknis yang
kontekstual.

- Generik: "Teknologi ini membawa banyak manfaat bagi perusahaan."
- Spesifik: "Latency turun dari 800ms ke 220ms setelah caching layer-nya
  dipindah ke edge."

## 5. Teknis vs formal vs santai — cara milih di titik mana

Gunakan tabel ini sebagai heuristik cepat per jenis kalimat dalam artikel:

| Jenis kalimat | Register |
|---|---|
| Menyatakan fakta/data/klaim yang bisa diverifikasi | Formal, presisi, tanpa hedge berlebih |
| Menjelaskan konsep teknis ke audiens campuran | Teknis tapi dengan analogi/istilah dijelasin sekali di awal |
| Transisi/penghubung antar ide | Sedikit conversational, hindari jembatan formula |
| Opini/interpretasi penulis | Boleh lebih personal, first-person kalau sesuai konteks (blog) |
| Pembuka/penutup | Paling fleksibel — di sinilah "suara" penulis paling kelihatan |

## 7. Materi mentah vs referensi yang dikutip

Kalau user kasih file lokal (draft kasar, notes, transkrip, hasil brain dump, dokumen internal) sebagai bahan tulisan — perlakukan itu sebagai **bahan mentah untuk dikembangkan**, bukan sebagai sumber yang dikutip di artikel final.

Aturan:

- **Jangan referensikan file lokal di artikel final.** Tidak ada bagian "Referensi: berdasarkan dokumen X.md" atau link `file://` ke path lokal. Pembaca artikel tidak pernah bisa mengakses file itu — mencantumkannya sebagai sumber itu useless sekaligus aneh (kok artikel publik referensinya dokumen pribadi penulis sendiri).
- **Kembangkan isinya, bukan cuma rapikan.** Draft kasar biasanya berupa poin-poin atau tulisan belum matang. Tugasnya nulis ulang jadi prosa yang mengalir sesuai prinsip di skill ini (register-shifting, spesifisitas, dst) — bukan sekadar copy-paste dengan sedikit polesan lalu ditempel link ke sumber aslinya.
- **Kalau artikel butuh menyebut sumber**, gunakan sumber yang publicly accessible dan bisa diverifikasi pembaca: dokumentasi resmi (mis. docs resmi suatu library/framework), artikel/publikasi yang bisa diakses publik, buku, paper, atau situs organisasi terkait. Jangan mengarang sumber atau nyebut sesuatu "menurut studi X" kalau tidak benar-benar ada dan bisa diverifikasi.
- **Kalau tidak ada sumber publik yang relevan atau pasti**, lebih baik tidak usah pakai section "Referensi/Sumber" sama sekali daripada menunjuk ke file lokal yang tidak bisa diakses siapa-siapa selain penulis.
- Ini juga berlaku untuk knowledge dari percakapan sebelumnya, catatan pribadi user, atau konteks internal lain yang dikasih ke penulis — semua itu bahan konteks untuk nulis, bukan citation.

### Signature style sitasi: inline hyperlink, bukan footnote atau daftar pustaka

Skill ini pakai **satu gaya sitasi konsisten di semua format** (Medium, corporate blog, tutorial, news): link ditempel langsung inline di kata/istilah yang relevan, bukan dikumpulkan jadi nomor bracket ala Wikipedia (`[1]`, `[2]`) atau daftar pustaka di akhir tulisan ala paper/skripsi.

Kenapa ini yang dipilih sebagai default, bukan dua yang lain:

- Nomor bracket (Wikipedia-style) berasa encyclopedic — mismatch sama tone professional-tapi-manusiawi yang jadi target skill ini.
- Daftar pustaka di akhir (academic/buku-style) bikin friction: pembaca artikel web ekspektasinya klik langsung pas ketemu klaim yang perlu verifikasi, bukan scroll ke bawah cari nomor lalu balik lagi ke posisi baca.
- Inline link adalah convention yang sudah dipakai tulisan teknis web pada umumnya (blog engineering, dokumentasi, tutorial) — jadi paling "universal" buat lintas format yang skill ini cover.

Praktiknya:

- **Nama tool/library/framework spesifik** → link-kan di first mention ke sumber resminya: halaman npm (`npmjs.com/package/...`), repo GitHub/GitLab resminya, atau docs resmi. Contoh: `[Resilience4j](https://github.com/resilience4j/resilience4j)`, `[ioredis](https://www.npmjs.com/package/ioredis)`.
- **Angka/klaim benchmark** → link ditempel dekat angkanya, bukan di catatan kaki terpisah. Contoh: "...turun ke 340ms, sejalan dengan hasil [benchmark TechEmpower](https://...) untuk pola serupa" — bukan "...turun ke 340ms [3]" dengan `[3]` dijelasin di bawah.
- **Jangan over-link.** Cuma link istilah yang memang butuh rujukan eksternal (nama produk spesifik, angka yang diklaim dari sumber lain, standar/spek teknis). Kata umum atau konsep dasar (misal "database", "API") nggak perlu di-link.
- **Jangan mengarang URL.** Kalau nggak yakin URL persisnya atau nggak bisa diverifikasi (mis. lagi nggak ada akses browsing), jangan pasang link palsu — sebut nama sumbernya secara teks biasa tanpa hyperlink, atau cari dulu link yang benar sebelum dipasang.
- **Satu istilah cukup di-link sekali** per artikel, biasanya di first mention — jangan link ulang tiap kali nama yang sama disebut lagi.

## 8. Contoh artikel per format

Folder `examples/` isinya contoh artikel lengkap yang udah mengikuti prinsip-prinsip di skill ini, dipecah per gaya/format supaya tulisan nggak monoton dan bisa disesuaikan sama kebutuhan user:

- `examples/medium-personal-blog.md` — gaya Medium/blog personal: first-person, naratif, opini penulis kental, pembuka cerita konkret.
- `examples/professional-corporate-blog.md` — gaya blog perusahaan/produk (engineering blog, SaaS): terstruktur, ada framing masalah-solusi, tapi tetap manusiawi, bukan white paper.
- `examples/tutorial-howto.md` — gaya tutorial/how-to: langkah bernomor, prasyarat, code block, ekspektasi jelas di tiap step.
- `examples/news-report.md` — gaya berita/reportase: inverted pyramid, lead di atas, kutipan naratif, third-person, netral.

Sebelum nulis artikel, cek dulu format apa yang paling cocok sama request user (kalau user nggak spesifik, tanya atau infer dari konteks — misal "tulis artikel buat blog engineering kami" → professional-corporate-blog, "buatin panduan install X" → tutorial-howto). Baca contoh yang relevan buat kalibrasi nada dan struktur sebelum mulai nulis, jangan asal pukul rata satu gaya buat semua jenis request.

### Yang boleh ditiru dari contoh, dan yang harus dihindari

"Kalibrasi nada dan struktur" itu maksudnya level abstrak, bukan nyontek plot. Contoh di `examples/` cuma satu buah per format — kalau dibaca sebagai template plot alih-alih template gaya, hasilnya artikel-artikel berikutnya bakal punya arc cerita, urutan argumen, atau bahkan kalimat penutup yang mirip banget satu sama lain, meski topiknya beda. Itu justru bikin tulisan kerasa formulaic, kebalikan dari tujuan skill ini.

**Boleh ditiru** (level gaya/struktur, abstrak):
- Rasio formal-vs-santai di tiap bagian (pembuka santai, badan presisi, penutup agak formal).
- Panjang dan ritme paragraf/kalimat.
- Bagaimana heading/bullet dipakai (atau sengaja tidak dipakai) di format tersebut.
- Cara nempelin inline citation (section 7).
- Struktur frontmatter (section 9).

**Harus dihindari** (level konten/plot, konkret — ini yang bikin dua artikel beda topik kerasa "sama"):
- Arc naratif yang sama persis (mis. "insiden malam hari → titik balik → gambar ulang di whiteboard → angka before/after → pertanyaan reflektif → closing soal masih nyimpen diagram lama"). Draft/brief dari user yang nentuin plotnya, bukan contoh.
- Kalimat pembuka atau penutup yang cuma ganti variabel dari contoh (angka, nama service, jumlah tim) tapi kerangka kalimatnya identik.
- Framing masalah atau analogi spesifik yang dipakai di contoh (mis. analogi sekering listrik, analogi tim vs organisasi) — kalau topik baru butuh analogi, cari yang orisinal buat konteks itu.

Kalau user kasih draft/brief sendiri (kayak di section 7), susun dulu kerangka/beat cerita dari draft itu sebelum buka contoh di `examples/` — supaya contoh cuma jadi pengecekan nada di akhir, bukan starting point buat mikirin plot.



Setiap artikel final dari skill ini **wajib** dibuka dengan YAML frontmatter sebelum H1, formatnya:

```
---
name: ${TITLE}
description: ${META_DESCRIPTION}
tag: ${TAG1}, ${TAG2}, ${TAG3}
---

# ${Judul}
```

Ini berlaku di semua format (Medium, corporate blog, tutorial, news) — strukturnya sama, isinya yang menyesuaikan konten.

Aturan tiap field:

- **name**: judul artikel. Senada dengan H1 di bawahnya — boleh beda sedikit kalau H1 versi lebih naratif dan `name` versi lebih ringkas buat metadata, tapi jangan sampai beda topik/fokus.
- **description**: satu kalimat ringkas ala meta description SEO, idealnya 120–160 karakter. Isinya intisari kenapa artikel ini layak dibaca, bukan cuma ngulang judul dengan kata lain. Hindari basa-basi kosong ("Artikel ini akan membahas tentang...") — langsung ke substansi/hasil/insight-nya.
- **tag**: 3–6 keyword relevan, dipisah koma, lowercase konsisten (mis. `circuit-breaker, resilience, microservices, sistem-terdistribusi`). Ini buat kategorisasi/SEO, bukan hashtag sosmed — hindari tag generik ("teknologi", "tips") kalau ada opsi yang lebih spesifik ke topik artikel.

Contoh:

```
---
name: Circuit Breaker: Menghentikan Kegagalan Sebelum Menjadi Bencana
description: Cara circuit breaker mencegah satu dependensi yang gagal menyeret seluruh sistem, lengkap tiga status, parameter kunci, dan studi kasus payment gateway.
tag: circuit-breaker, resilience, microservices, sistem-terdistribusi, reliability
---

# Circuit Breaker: Menghentikan Kegagalan Sebelum Menjadi Bencana
```

## 10. Checklist sebelum selesai

- [ ] Ada paragraf pembuka generik yang bisa dipakai di artikel topik lain? → ganti jadi spesifik.
- [ ] Ada pola "not just X but Y" / "bukan X, tapi Y" / rule-of-three yang berulang? → variasikan.
- [ ] Semua kalimat panjangnya mirip-mirip (semua pendek atau semua panjang)? → selingi.
- [ ] Kesimpulan cuma ngulang isi? → tambahin implikasi baru.
- [ ] Bagian teknis: istilah dijelasin sekali di awal, konsisten dipakai (jangan ganti-ganti istilah untuk hal yang sama).
- [ ] Heading/bullet dipakai secukupnya, bukan buat memecah semua paragraf jadi list.
- [ ] Ada bagian "Referensi/Sumber" yang nunjuk ke file lokal atau link `file://`? → hapus; ganti sumber publik yang valid atau hilangkan section-nya.
- [ ] Ada YAML frontmatter (name/description/tag) di paling atas file, sebelum H1?
- [ ] Arc cerita, framing masalah, atau kalimat penutup mirip banget sama salah satu contoh di `examples/` (cuma beda angka/nama)? → tulis ulang plotnya dari draft/brief user, bukan dari contoh.
- [ ] Baca ulang: kalau dibaca keras, apakah kedengeran kayak orang cerita, atau kayak dokumen yang disusun poin per poin?

## 11. Thumbnail

Jika kamu punya akses ke image generator tools, buat 1 atau lebih gambar pendukung untuk artikel yang telah kamu buat