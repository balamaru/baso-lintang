# AGENTS.md

Panduan untuk agen AI (Codex, Claude Code, dan sejenisnya) yang bekerja di repositori ini.
Baca seluruh file ini sebelum mengubah kode. Sumber kebenaran produk dan desain ada di
`docs/PRD.md`, `docs/SYSTEM_DESIGN.md`, dan `docs/DOKUMEN_PENDUKUNG.md`.
Jika file ini dan dokumen itu bertentangan, ikuti dokumen itu lalu beri tahu pengguna agar file ini diperbarui.

## 1. Tentang proyek

Kamus terbuka untuk **bahasa Lintang**, bahasa masyarakat adat yang belum punya referensi tertulis.
Siapa pun bisa mencari dan menambah kata **tanpa akun**. Hanya tim (admin dan verifikator) yang punya akun.
Kiriman publik masuk karantina (`submissions`, status `pending`) dan baru menjadi kata resmi (`entries`)
setelah diverifikasi manusia.

Ini data budaya komunitas, bukan data biasa. Perlakukan dengan hati-hati (lihat bagian 4 dan 8).

Status saat ini: **Versi 0.1 (draf), tahap M0 (persiapan)**. Fokus kerja adalah **MVP (Fase 1, semua kebutuhan P0)**.
Jangan membangun fitur Fase 2 ke atas kecuali diminta eksplisit.

## 2. Teknologi

| Lapisan | Pilihan |
| --- | --- |
| Bahasa | Go (versi stabil terbaru) |
| Router | `chi` |
| Tampilan | `html/template` (render di server) + htmx + Tailwind CSS (build statis). **Tanpa framework JS berat** |
| Database | PostgreSQL dengan ekstensi `pg_trgm`, `unaccent`, `citext` |
| Akses data | `sqlc` + `pgx` (SQL ditulis manual, kode Go dihasilkan) |
| Migrasi | `goose` |
| Sesi | Cookie sesi, disimpan di tabel `sessions` (tanpa Redis) |
| Hash password | `argon2id` |
| CAPTCHA | Cloudflare Turnstile |
| Audio (Fase 3) | MinIO / S3-kompatibel |
| Log | `slog` JSON terstruktur |
| Konfigurasi | Environment variable |

Arsitektur: **monolit modular, satu binary Go, satu database**. Jangan menambah microservice, message queue,
Redis, Elasticsearch, atau ORM. Jika merasa perlu salah satunya, berhenti dan tanyakan dulu (lihat ADR 003, 004).

## 3. Struktur repositori

```
cmd/app/                 main.go, admin_cli.go (admin create, migrate, export)
internal/config/         baca env
internal/http/           router.go, middleware/ (ratelimit, csrf, session, logging, recover)
internal/http/public/    handler publik
internal/http/admin/     handler admin
internal/domain/         logika: submission, entry, search, normalize
internal/store/          hasil sqlc + repositori
internal/auth/           argon2id, sesi, undangan
internal/antispam/       turnstile, honeypot, limiter
internal/storage/        klien S3/MinIO
internal/jobs/           purge hash lama, trigger backup
db/migrations/           file goose
db/queries/              file .sql untuk sqlc
web/templates/           html/template
web/static/              css, js htmx, ikon
deploy/                  Dockerfile, docker-compose.yml, k8s/
docs/                    PRD, ADR, runbook
```

Aturan lapisan: handler HTTP tipis, logika bisnis di `internal/domain`, akses database hanya lewat `internal/store`.
`domain` tidak boleh mengimpor `http`.

## 4. Aturan bisnis yang TIDAK boleh dilanggar

Ini invarian produk. Setiap perubahan kode harus menjaganya, dan setiap perubahan perlu tes yang membuktikannya.

1. **BR-1.** Kiriman anonim tidak pernah tampil publik sebelum disetujui. Tidak ada jalur kode yang menulis dari
   `/kirim` langsung ke `entries`, apa pun kondisinya (termasuk saat Turnstile tidak tersedia).
2. **BR-2.** Dukungan anonim hanya sinyal; tidak mengubah status entri.
3. **BR-3.** Verifikator tidak boleh menyetujui kiriman dari sesi loginnya sendiri.
4. **BR-4.** Setiap entri wajib punya minimal satu ejaan primer dan satu arti (`senses`).
5. **BR-5 / BR-6.** `origin` dan `relation` hanya diisi verifikator. Nilai yang sah diatur CHECK constraint di skema.
6. **BR-7.** `ip_hash` dan `fingerprint_hash` dihapus otomatis setelah 90 hari (pekerjaan di `internal/jobs`).
7. **BR-8.** Kontak penyumbang tidak pernah tampil publik. Nama hanya tampil jika `credit_consent = true`.
8. Tidak ada rute atau tombol pendaftaran publik. Akun hanya lahir dari undangan admin (token 32 byte acak,
   simpan hash, kedaluwarsa 72 jam) atau CLI `app admin create`.
9. Entri `visibility = hidden` atau `is_sensitive = true` **tidak boleh** muncul di pencarian publik, daftar, halaman detail,
   ekspor publik, maupun API. Uji ini secara eksplisit.
10. Ejaan **tidak dibakukan**. Simpan varian di `spellings`. Aturan normalisasi (huruf yang sering bervariasi) harus
    berupa konfigurasi, bukan hardcode.

## 5. Privasi dan keamanan

- **Jangan** menyimpan, mencatat di log, atau menampilkan IP mentah. Hanya hash bergaram (`HASH_SALT`).
- Jangan mencatat isi kiriman penuh di log. Jangan menaruh kontak penyumbang di log, error, atau metrik.
- Tanpa pelacak pihak ketiga selain Turnstile. Jangan menambah analytics, font CDN, atau skrip eksternal tanpa persetujuan.
- Semua query terparameter lewat `sqlc`/`pgx`. **Dilarang** merangkai SQL dengan string.
- `html/template` untuk semua keluaran. Dilarang `template.HTML` / `template.JS` pada data dari pengguna.
- Cookie sesi: `HttpOnly; Secure; SameSite=Lax`, simpan hash token di DB, rotasi saat login.
- CSRF token wajib di setiap form POST di `/admin`.
- Login: argon2id, kunci 15 menit setelah 5 gagal. Pesan galat tidak membedakan "email salah" dan "password salah".
- Header: `Content-Security-Policy` ketat, `X-Content-Type-Options`, `Referrer-Policy`, `Strict-Transport-Security`,
  `Permissions-Policy` (mikrofon hanya di halaman rekam).
- Rahasia hanya lewat env. **Jangan pernah commit** `.env`, kunci, atau dump database. Contoh nilai taruh di `.env.example`.
- Pin versi dependensi. Jangan menambah dependensi baru tanpa alasan jelas; sebutkan alasannya di PR.

## 6. Alur kiriman (`POST /kirim`)

Urutan pemeriksaan harus persis begini (murah ke mahal, dan gagal cepat):

1. Cek `blocklist` dengan hash IP
2. Rate limit per hash IP dan per sidik perangkat (awal: 10/jam, 30/hari, konfigurasi lewat `RATE_LIMIT_*`)
3. Validasi token Turnstile ke server Cloudflare
4. Honeypot: field tersembunyi harus kosong
5. Validasi isi: kata maks 60 karakter, penjelasan maks 1.000 karakter, tolak URL dan email di field kata, tolak karakter kontrol
6. Duplikat kasar: ejaan + arti sama pada kiriman `pending` dalam 24 jam, tampilkan pesan "sudah diterima"
7. Simpan sebagai `pending`

Form harus tetap terisi saat ada galat validasi dan harus berfungsi tanpa JavaScript berat.

## 7. Pencarian

- Normalisasi di aplikasi: huruf kecil, hapus aksen dan tanda baca. Simpan di kolom `normalized`.
- `arah=lintang`: cocok persis pada `spellings.normalized`, lalu fuzzy `similarity(normalized, q) > 0.3` (`pg_trgm`).
- `arah=indonesia`: FTS pada `senses.search_vec`, dilengkapi trigram pada `gloss_id`; `gloss_id` sama persis diberi bobot lebih.
- Urutan: kecocokan persis, kemiripan, lalu entri dengan dukungan atau audio lebih banyak.
- Hasil kosong (Fase 3): catat atau increment `failed_searches` (hanya `query_norm` dan `direction`, tanpa data pengunjung).
- Target: p95 di bawah 300 ms untuk 50.000 entri. Periksa `EXPLAIN ANALYZE` untuk query pencarian baru.

## 8. Konvensi kode

- Format dengan `gofmt`/`goimports`; lolos `go vet` dan `golangci-lint` bila dikonfigurasi.
- Error: bungkus dengan konteks (`fmt.Errorf("...: %w", err)`), jangan menelan error, jangan `panic` di jalur request.
- Setiap fungsi yang menyentuh DB menerima `context.Context`.
- Gunakan `slog`, bukan `fmt.Println`/`log.Printf`.
- Migrasi: file `goose` baru, bergantian naik/turun, **jangan mengedit migrasi yang sudah dirilis**. Setelah mengubah
  `db/queries/*.sql` atau skema, jalankan `sqlc generate` dan commit hasilnya.
- Jangan mengedit berkas hasil generate secara manual.
- Nama identifier, kolom DB, dan komentar kode dalam **bahasa Inggris** (sesuai skema di PRD).
  Teks antarmuka, pesan galat untuk pengguna, dan isi dokumen dalam **bahasa Indonesia**.
  Path route publik tetap dalam bahasa Indonesia (`/kirim`, `/cari`, `/kata/{slug}`, `/perlu-diisi`, `/tentang`).
- Antarmuka: HP dulu, kontras dan ukuran huruf memadai, bisa dipakai keyboard. Penggunanya mencakup penutur lanjut usia.
- Tailwind di-build statis; jangan memuat dari CDN.
- Pesan commit: Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`).
  Satu perubahan logis per commit.

## 9. Perintah umum

> Makefile backend tersedia. Prasyarat dan konfigurasi HTTPS lokal ada di README.md.

```bash
make dev           # docker compose: app + migrasi + Postgres
make migrate-up    # app migrate up
make migrate-down
make sqlc          # sqlc generate
make test          # go test ./...
make test-int      # tes integrasi (testcontainers-go + Postgres)
make lint          # go vet (golangci-lint belum dikonfigurasi)
make sec           # gosec + govulncheck
make build         # build binary Go
```

Target CSS belum tersedia; tampilan saat ini form SSR dasar untuk pengujian backend.

Jika ragu perintah mana yang berlaku, baca `Makefile` dan `deploy/docker-compose.yml`, jangan menebak.

## 10. Pengujian

Sebelum menganggap tugas selesai, jalankan tes yang relevan dan laporkan hasilnya apa adanya (jangan klaim lulus tanpa menjalankan).

| Jenis | Cakupan | Alat |
| --- | --- | --- |
| Unit | normalisasi ejaan, validasi form, hash, limiter | `go test` |
| Integrasi | query sqlc, pencarian trigram/FTS, transisi status | `testcontainers-go` + Postgres |
| E2E | kirim, antrean, terima, tampil di pencarian; undangan, login | Playwright atau uji HTTP |
| Keamanan | XSS, CSRF, akses per role, brute force | `gosec`, `govulncheck`, uji manual |

Setiap perubahan pada aturan bisnis (bagian 4) wajib disertai tes. Contoh tes yang harus ada:
kiriman anonim tidak muncul di pencarian; verifikator tidak bisa menyetujui kiriman sendiri; entri tersembunyi
tidak bocor di route publik; verifier tidak bisa membuka `/admin/pengguna`.

Kriteria rilis MVP: semua tes unit dan integrasi lulus, tidak ada celah keamanan tingkat tinggi, uji pulih cadangan sukses.

## 11. Cara bekerja

1. Mulai dengan membaca bagian PRD/desain yang relevan dengan tugas (ID `FR-xx`, `BR-x`, bagian 2.x).
2. Sebutkan ID kebutuhan yang dikerjakan dalam deskripsi PR/commit, misalnya `feat: form kirim kata (FR-06, FR-08)`.
3. Kerjakan perubahan **kecil dan terfokus**. Jangan refactor di luar lingkup tugas.
4. Tahan diri dari memperluas lingkup: fitur yang bertanda P1/P2 tidak dibangun di MVP.
5. Jika ada pertanyaan terbuka di PRD 1.13 (lisensi, kategori kata terlarang, hosting, dll.) yang memengaruhi tugas,
   **tanyakan pengguna**; jangan memutuskan sendiri.
6. Catat keputusan arsitektur baru sebagai ADR di `docs/adr/`.

## 12. Yang perlu persetujuan manusia sebelum dikerjakan

- Mengubah skema yang mengubah makna data, atau menghapus kolom/tabel
- Mengubah aturan bisnis di bagian 4, alur anti-spam, autentikasi, atau penyimpanan data pribadi
- Menambah dependensi besar, layanan eksternal, atau pelacak
- Menentukan lisensi data, kebijakan privasi, atau teks yang menyangkut adat dan kata sensitif
- Menjalankan perintah destruktif pada data (drop, truncate, restore) atau menyentuh lingkungan produksi

## 13. Data dan konten bahasa

- **Jangan mengarang kosakata Lintang.** Jangan membuat contoh kata, arti, atau kalimat Lintang "yang terdengar benar"
  di seed, fixture, atau dokumentasi. Gunakan placeholder yang jelas palsu (misalnya `kata-uji-1`) untuk data uji,
  dan tandai di komentar bahwa itu bukan bahasa Lintang.
- Jangan membuat klaim tentang asal kata, hubungan dengan Melayu/Indonesia, atau tata bahasa Lintang.
  Itu wewenang verifikator dan penutur.
- Jangan "membenahi" ejaan atau menyeragamkannya dalam kode atau data.
- Jangan menyalin data kamus ke luar sistem (misalnya ke layanan AI pihak ketiga) tanpa izin pengelola.

## 14. Definisi selesai (checklist)

- [ ] Perubahan sesuai kebutuhan yang dirujuk (FR/BR) dan tidak melebihi lingkup
- [ ] Invarian bagian 4 tetap terjaga, ada tes bila relevan
- [ ] `go vet`, tes terkait, dan `sqlc generate` (jika SQL berubah) sudah dijalankan; hasil dilaporkan jujur
- [ ] Tidak ada rahasia, IP mentah, atau data pribadi di kode, log, atau fixture
- [ ] Migrasi baru dapat naik dan turun
- [ ] Dokumen/ADR diperbarui bila perilaku atau keputusan berubah


# Codex Development Instructions

## General
- Implement the user's requested changes.
- Follow existing project conventions.
- Do not modify unrelated files.

## Git Workflow
- (One time) Create a feature branch
- Work only on a feature branch.
- Never commit or push directly to main or master.
- Never force push.
- Never use git reset --hard without approval.
- Never commit secrets, tokens, or .env files.
- Never stage unrelated changes.

## Validation
- Run relevant tests before committing.
- Run lint and typecheck when available.
- If tests fail, fix the issues before committing.
- If validation cannot run, report the blocker.

## Automatic Commit and Push
After completing a requested coding task:

1. Check git status.
2. Review git diff.
3. Stage only files related to the task.
4. Create a conventional commit.
5. Push the current feature branch to origin.
6. Request permission when Codex requires approval.
7. Report commit hash, branch, and push status.

## Commit Message Format
- feat: new feature
- fix: bug fix
- refactor: code refactoring
- docs: documentation changes
- chore: maintenance

## Restrictions
- Do not bypass branch protection.
- Do not use force push.
- Do not push when tests fail.
- Do not automatically merge pull requests.
- Do not claim a push succeeded without verifying it.

<!-- session-memory:start -->
## Memori session (session-memory)
Proyek ini memakai `.agent-memory/` sebagai memori lintas session & lintas agent.
1. **Awal session, SEBELUM mengerjakan prompt apa pun**: jika konteks belum berisi
   `=== SESSION MEMORY`, jalankan `python .agent-memory/bin/memory.py start --agent <nama-agent>`. Cocokkan prompt
   dengan requirements & session terakhir di output-nya. Jangan membaca ulang seluruh kode:
   baca hanya file yang relevan dengan task dan file yang ditandai berubah.
2. **Selama session**: `python .agent-memory/bin/memory.py log <task|done|decision|todo|blocker|fix|note> "teks singkat"`
   setiap ada langkah berarti; `python .agent-memory/bin/memory.py req set R<n> <x|~|-> "bukti"` saat requirement maju;
   `python .agent-memory/bin/memory.py files add <path> "fungsi"` untuk file penting baru.
3. Perbarui bagian manual `.agent-memory/MEMORY.md` bila status/keputusan/langkah berikutnya berubah.
4. **Akhir session** (task tuntas / user pamit): `python .agent-memory/bin/memory.py end --summary "ringkasan 2–4 kalimat"`.
   Bila lebih dari satu agent aktif, tambahkan `--agent <nama-agent>` yang sama di `log`/`end`.
5. Jangan pernah menulis rahasia (API key, token, password) atau data pribadi ke memori.
Detail protokol: `.agent-memory/PROTOCOL.md` (gunakan `python3` bila `python` tidak ada).
<!-- session-memory:end -->
