---
name: session-memory
description: Memori persisten lintas-session dan lintas-agent untuk coding agent (Claude Code, Codex, Gemini CLI, Cursor, Copilot, Aider, Cline, dll.), disimpan di folder `.agent-memory/` proyek. Gunakan skill ini di AWAL setiap session coding — sebelum mengerjakan prompt apa pun — untuk memuat ringkasan requirements awal, progres, keputusan, dan session sebelumnya alih-alih membaca ulang seluruh kode; gunakan juga SELAMA session untuk mencatat progres dan di AKHIR session untuk menyimpan ringkasan. Picu juga ketika user meminta melanjutkan pekerjaan sebelumnya ("lanjutkan yang kemarin", "resume", "continue where we left off"), meminta agent mengingat/menyimpan progres atau session, menyebut handoff antar agent, memori proyek, hemat token konteks, ingin menyiapkan memori untuk proyek baru, atau ketika folder `.agent-memory/` atau blok `session-memory` di AGENTS.md/CLAUDE.md ada di proyek.
---

# Session Memory

Memori proyek yang ringkas dan selalu mutakhir, dipakai bersama oleh semua coding agent.
Tujuannya: session baru langsung tahu *apa yang diminta* (requirements awal), *apa yang sudah
dikerjakan* (session sebelumnya), *apa yang diputuskan dan mengapa*, serta *file mana yang
perlu dibaca* — dengan biaya ~1–3 ribu token, bukan puluhan ribu token untuk membaca ulang kode.

Semua logika ada di `scripts/memory.py` (Python 3.8+, stdlib saja, Windows/macOS/Linux).
Saat `init`, skrip disalin ke `<proyek>/.agent-memory/bin/memory.py`, sehingga agent yang tidak
mendukung skill pun bisa memakainya lewat `AGENTS.md`.

Di bawah, `M` berarti `python .agent-memory/bin/memory.py` (gunakan `python3` bila `python`
tidak ada; di Windows bisa juga `py -3`).

## Kapan melakukan apa

Tentukan dulu situasinya:

- **`.agent-memory/` belum ada** → lakukan *Penyiapan awal* di bawah, lalu lanjut ke *Awal session*.
- **Konteks sudah berisi `=== SESSION MEMORY`** (dimuat hook Claude Code) → briefing sudah ada;
  langsung ke langkah 3 *Awal session*.
- **Selain itu** → jalankan *Awal session* dari langkah 1.

### Penyiapan awal (sekali per proyek)
1. `python <folder-skill-ini>/scripts/memory.py init` dari root proyek. Tambahkan
   `--req <path>` bila dokumen requirements tidak terdeteksi otomatis (skrip mencari nama seperti
   `goals.md`, `requirements.md`, `PRD.md`, `spec*.md`). Bila tidak ada dokumen sama sekali,
   tanyakan user di mana requirements-nya atau minta ringkasannya.
2. Baca dokumen requirements **sekali** dan isi `.agent-memory/requirements.md` dengan baris
   `- [ ] R1: <kebutuhan singkat yang bisa diuji> — <catatan>`. Beri status yang sesuai kenyataan
   (`[x]` selesai, `[~]` sebagian) bila proyek sudah berjalan.
3. Isi bagian manual `.agent-memory/MEMORY.md` (tujuan, status, keputusan, konvensi build/test,
   langkah berikutnya, jebakan). Bila sudah ada catatan proyek (notes, CLAUDE.md lama, README,
   CHANGELOG), ringkas dari sana — lebih murah daripada menyimpulkan dari kode. Contoh hasil yang
   baik: `references/example-memory.md`.
4. Daftarkan 10–40 file inti: `M files add <path> "fungsi singkat"`. Peta ini menyimpan hash isi,
   sehingga session berikutnya tahu file mana yang berubah dan perlu dibaca ulang.
5. Sampaikan ke user apa yang dibuat `init` (file bootstrap dan hook yang ditambahkan).

### Awal session — sebelum mengerjakan prompt
1. `M start --agent <nama>` — nama: `claude-code`, `codex`, `gemini-cli`, `cursor`, `copilot`,
   `aider`, dst. Ini menutup otomatis session lama yang lupa ditutup, membuka session baru,
   dan mencetak briefing (MEMORY.md, requirements yang belum selesai, perubahan git, file di peta
   yang berubah, session agent lain yang terbuka).
2. Baca briefing-nya. Jangan membuka file kode dulu. Perintah di briefing sudah menyertakan
   `--agent <nama>`; pakai bentuk yang sama untuk `log`/`end` agar tidak tertukar dengan session
   agent lain yang mungkin sedang terbuka.
3. Cocokkan prompt user dengan briefing:
   - Requirement mana yang disentuh? Sudah selesai, sebagian, atau belum?
   - Apakah prompt melanjutkan "Langkah berikutnya"/TODO, atau bertentangan dengan keputusan
     yang tercatat? Bila bertentangan, sampaikan singkat ke user sebelum mengubah arah.
   - File mana yang benar-benar perlu dibaca? Gunakan peta kode dan daftar file berubah;
     baca hanya itu, lalu `M files refresh <path>` untuk file peta yang sudah dibaca ulang.
4. `M log task "<prompt user, 1 kalimat>"`, lalu kerjakan task-nya.

Pemeriksaan ini sebaiknya singkat dan tidak dinarasikan panjang ke user — cukup sebut hal yang
relevan (mis. "R4 sudah selesai di session kemarin; yang tersisa validasi input").

### Selama session — perbarui memori terus
Catat saat sesuatu terjadi, bukan menunggu akhir: session bisa terputus kapan saja, dan catatan
yang ditunda biasanya hilang.

| Kejadian | Perintah |
|---|---|
| Unit kerja selesai | `M log done "tambah retry di notifier.py (backoff 3x)"` |
| Keputusan + alasan | `M log decision "pakai stdlib saja — image kecil & tanpa CVE dependensi"` |
| Sisa pekerjaan / hambatan | `M log todo "..."` · `M log blocker "..."` |
| Bug diperbaiki | `M log fix "dedup gagal saat API down — state tidak di-resolve"` |
| Fakta berguna | `M log note "test: python -m unittest discover tests"` |
| Requirement maju | `M req set R3 x "tests/test_logscan.py lulus"` · `M req set R5 ~ "auth belum"` |
| Kebutuhan baru dari user | `M req add "..."` |
| File inti baru | `M files add src/x.py "fungsi"` |

Edit bagian manual `MEMORY.md` saat status, keputusan penting, atau langkah berikutnya berubah —
edit di tempat, jangan menumpuk riwayat (riwayat sudah ada di `sessions/`).

### Akhir session
Saat task tuntas, user pamit, atau user meminta menyimpan progres:
1. Pindahkan TODO yang belum selesai ke "Langkah berikutnya" di `MEMORY.md`; hapus yang sudah selesai.
2. `M end --summary "apa yang dicapai, keadaan kode sekarang, apa berikutnya (2–4 kalimat)"`.
3. Daftarkan file baru penting yang dilaporkan `end`.

Bila `end` tidak sempat dijalankan, session ditutup otomatis (hook `SessionEnd` Claude Code, atau
`start` berikutnya) dengan ringkasan otomatis dari log + diff git — tetapi ringkasan dari agent
selalu lebih berguna.

## Aturan menulis memori
- **Tanpa rahasia dan data pribadi.** Jangan pernah menulis API key, token, password, isi
  `.env`/Secret, string koneksi berkredensial, email atau data pribadi. Sebut nama variabelnya
  (`WEBHOOK_AUTH_TOKEN`), bukan nilainya. Skrip menyamarkan pola umum, tapi itu jaring pengaman,
  bukan izin.
- **Ringkas.** `MEMORY.md` < ~150 baris; satu baris log 10–25 kata. Briefing yang gemuk
  menghapus penghematan token yang menjadi alasan skill ini ada.
- **Tulis yang tidak terlihat dari kode**: alasan keputusan, pendekatan yang gagal, konvensi,
  status requirement, rencana. Jangan menyalin kode; rujuk path dan nama fungsi.
- **Bahasa** mengikuti user/dokumen proyek.

## Referensi perintah
`M -h` untuk detail. Ringkas: `init`, `start`, `brief`, `log <tipe> "..."`, `checkpoint`,
`end --summary`, `req list|add|set`, `files list|add|rm|refresh|stale`, `compact --keep N`, `status`.
`compact` berjalan otomatis saat `end` (menyimpan 8 session detail terbaru; sisanya satu baris
per session di `history.md`).

## File pendukung
- `references/protocol.md` — protokol lengkap; disalin ke `.agent-memory/PROTOCOL.md` untuk agent
  tanpa dukungan skill.
- `references/agents.md` — cara integrasi tiap agent (hook Claude Code, AGENTS.md untuk Codex,
  GEMINI.md, Cursor rules, Copilot, Aider), handoff antar agent, dan saran git. Baca bila user
  memakai agent selain Claude Code atau bertanya soal pemasangan.
- `references/example-memory.md` — contoh `MEMORY.md` dan `requirements.md` yang terisi baik.
