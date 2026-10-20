# Protokol session-memory (untuk semua coding agent)

Dokumen ini disalin ke `.agent-memory/PROTOCOL.md` saat `init`, sehingga agent yang tidak
mendukung skill (Codex lama, Aider, Cline, dll.) tetap bisa mengikutinya. Perintah di bawah
memakai `python`; ganti dengan `python3` bila perlu. `M` = `python .agent-memory/bin/memory.py`.

## Mengapa ada protokol ini
Membaca ulang seluruh kode di awal session memakan puluhan ribu token dan tetap tidak memberi
tahu *mengapa* kode ditulis begitu, apa yang sudah diputuskan, atau apa yang belum selesai.
Briefing memori biasanya 1–3 ribu token dan menjawab pertanyaan-pertanyaan itu. Kode dibaca
hanya bila relevan dengan task, dan peta kode memberi tahu file mana yang berubah sejak
terakhir dibaca.

## Isi `.agent-memory/`
| File | Isi | Siapa yang menulis |
|---|---|---|
| `MEMORY.md` | Tujuan, status, keputusan, konvensi, langkah berikutnya, jebakan + blok `auto` session terakhir | Agent (manual) + skrip (blok auto) |
| `requirements.md` | Checklist R1..Rn turunan dari dokumen requirements awal, dengan status & bukti | Agent via `M req` |
| `filemap.json` | Peta file penting → fungsi + hash isi saat terakhir dibaca | Agent via `M files` |
| `sessions/*.md` | Log satu session (append-only) + file berubah + ringkasan | Skrip, dari `M log` / `M end` |
| `history.md` | Satu baris per session lama (hasil compact) | Skrip |
| `state.json` | Session yang sedang terbuka (lokal, di-gitignore) | Skrip |

## 1. Awal session — sebelum mengerjakan prompt
1. Bila konteks sudah berisi `=== SESSION MEMORY` (dimuat hook), lewati langkah 2.
2. Jalankan `M start --agent <nama>` (nama: `claude-code`, `codex`, `gemini-cli`, `cursor`,
   `copilot`, `aider`, …). Perintah ini menutup otomatis session lama yang lupa ditutup,
   membuka session baru, dan mencetak briefing.
3. Cocokkan prompt user dengan briefing sebelum menyentuh kode:
   - Requirement mana yang disentuh prompt ini? Apakah sudah selesai, sebagian, atau belum?
   - Apakah prompt melanjutkan TODO/langkah berikutnya, atau bertentangan dengan keputusan
     yang tercatat? Jika bertentangan, sebutkan ke user secara singkat sebelum mengubah arah.
   - File mana yang perlu dibaca? Gunakan peta kode + daftar file berubah; baca hanya itu.
4. Catat task: `M log task "<prompt user diringkas 1 kalimat>"`.

Hanya bila `MEMORY.md` kosong/belum berguna, atau requirements belum diisi, lakukan
penyiapan awal (lihat bagian 5) sebelum task.

## 2. Selama session — memori terus diperbarui
Catat begitu sesuatu terjadi, bukan menunggu akhir — session bisa terputus kapan saja.
- `M log done "..."` — unit kerja selesai (sebut file/fungsi utama).
- `M log decision "X dipilih karena Y"` — keputusan desain/teknis beserta alasannya.
- `M log todo "..."` / `M log blocker "..."` — pekerjaan tersisa / hambatan.
- `M log fix "..."` — bug yang diperbaiki + akar masalah singkat.
- `M log note "..."` — fakta yang berguna di masa depan (perintah test, jebakan lingkungan).
- `M req set R3 x "tests/test_x.py lulus"` · `M req set R5 ~ "endpoint ada, auth belum"`.
- `M req add "requirement baru dari user"` bila user menambah kebutuhan.
- `M files add src/foo.py "parser konfigurasi"` untuk file inti baru;
  `M files refresh src/foo.py` setelah membaca ulang file yang ditandai berubah.
- Edit bagian manual `MEMORY.md` saat status, keputusan penting, atau langkah berikutnya
  berubah. Edit di tempat; jangan menumpuk riwayat (riwayat ada di sessions/).

Satu baris log ≈ 10–25 kata. Tulis fakta yang tidak bisa disimpulkan dari kode/diff.

## 3. Akhir session
Ketika task tuntas, user pamit, atau user meminta menyimpan:
1. Pindahkan TODO yang belum selesai ke bagian "Langkah berikutnya" di `MEMORY.md`, dan
   hapus yang sudah selesai dari sana.
2. `M end --summary "apa yang dicapai, keadaan kode sekarang, apa berikutnya (2–4 kalimat)"`.
3. Bila `end` melaporkan file baru di luar peta, daftarkan yang penting.

Bila agent tidak sempat menjalankan `end`, session ditutup otomatis (hook SessionEnd di
Claude Code, atau `start` berikutnya) dengan ringkasan otomatis dari log + diff git.

## 4. Aturan menulis memori
- Tidak pernah: API key, token, password, isi `.env`/Secret, string koneksi berkredensial,
  email/nama/nomor pribadi. Skrip menyamarkan pola umum, tetapi jangan bergantung padanya.
  Sebut nama variabel (mis. `WEBHOOK_AUTH_TOKEN`), bukan nilainya.
- Ringkas: `MEMORY.md` < ~150 baris. Pindahkan detail usang ke `history.md` atau hapus.
- Faktual dan dapat diverifikasi: sebut path file, nama test, versi.
- Bahasa: ikuti bahasa yang dipakai user/dokumen proyek.
- Jangan menyalin blok kode panjang ke memori; rujuk path + nama fungsi.

## 5. Penyiapan awal (sekali per proyek)
1. `python <path-skill>/scripts/memory.py init` (atau `--req docs/goals.md` bila deteksi
   otomatis salah). Ini membuat `.agent-memory/`, menyalin skrip ke `.agent-memory/bin/`,
   menambah blok bootstrap ke `AGENTS.md`/`CLAUDE.md` (dan GEMINI.md, Cursor rules, dll.
   bila ada), serta memasang hook Claude Code.
2. Baca dokumen requirements sumber SEKALI dan isi `requirements.md`: satu baris per
   kebutuhan yang dapat diuji, `R1..Rn`, status awal sesuai kenyataan.
3. Isi bagian manual `MEMORY.md`. Bila sudah ada catatan (notes, CLAUDE.md lama, README,
   changelog), ringkas ke sini alih-alih membaca kode.
4. Daftarkan 10–40 file inti dengan `M files add`. Ini sekali-bayar: berikutnya agent hanya
   membaca file yang berubah.
5. `M start --agent <nama>` lalu kerjakan task user.

## 6. Beberapa agent sekaligus
Setiap agent memakai `--agent` berbeda sehingga log tidak tercampur. Briefing menampilkan
session agent lain yang masih terbuka. Session agent lain yang terbuka > 12 jam ditutup
otomatis saat ada `start` baru.
