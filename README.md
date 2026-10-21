# Backend Kamus Bahasa Lintang

Implementasi MVP sesuai `docs/PRD.md`: FR-01–08, FR-20–23, FR-30–31, BR-1/3/4/7/8.
Monolit Go, chi, SSR `html/template`, PostgreSQL, pgx, query hasil sqlc, migrasi goose.
Belum merupakan persetujuan untuk rilis publik.

## Menjalankan lokal

Prasyarat Compose: Docker Desktop/Engine dengan Compose v2; Python 3 untuk penyiapan
otomatis (opsional). Go 1.27.3 hanya diperlukan untuk menjalankan alat Go di host.

Jalankan dari root repositori (pada Linux/macOS gunakan `python3` bila diperlukan):

```sh
python scripts/setup-local.py
mkdir .cache
docker compose --env-file .env -f deploy/docker-compose.yml up --build -d --wait
docker compose --env-file .env -f deploy/docker-compose.yml run --rm admin --email admin@example.invalid --name "Admin uji"
docker compose --env-file .env -f deploy/docker-compose.yml cp https:/data/caddy/pki/authorities/local/root.crt .cache/local-ca.crt
```

Lewati `mkdir .cache` jika folder sudah ada.
Buka **https://localhost:8443**. Untuk browser, impor `.cache/local-ca.crt` ke trust store
sertifikat pengembangan lokal; untuk curl gunakan `--cacert .cache/local-ca.crt`.
Caddy menyediakan HTTPS lokal; tidak ada permintaan sertifikat publik atau access log.
Password admin ada pada `ADMIN_PASSWORD` di `.env`; setelah admin berhasil dibuat dan diuji,
kosongkan nilai itu. Service aplikasi tidak menerima password bootstrap tersebut.
Perintah pembuatan admin dijalankan sekali per alamat email.

Skrip setup tidak menimpa `.env`. Jika sudah ada, isi `POSTGRES_PASSWORD` dengan teks
acak heksadesimal (aman untuk URL database), `SESSION_SECRET` dan `HASH_SALT` minimal
32 karakter, serta pasangan kunci Turnstile pengembangan. Compose lokal menetapkan
origin `https://localhost:8443`. Jangan mengganti password PostgreSQL pada volume yang
sudah berisi data tanpa melakukan rotasi password database secara terencana.

PostgreSQL menyimpan data di named volume; migrasi selesai sebelum backend dimulai,
dan proxy menunggu backend sehat. Hanya port HTTPS 8443 yang diekspos, ke loopback.
Endpoint operasional dan database tidak dipublikasikan ke host. Untuk inspeksi SQL:
`docker compose --env-file .env -f deploy/docker-compose.yml exec db psql -U lintang -d lintang`.
Untuk status/log gunakan `ps` atau `logs --tail 50 app migrate` dengan prefix Compose yang sama.
Hentikan dengan `docker compose --env-file .env -f deploy/docker-compose.yml down`;
volume tetap tersimpan. Jangan memakai `down -v` jika data perlu dipertahankan.

Konfigurasi ini untuk pengembangan lokal. Limiter melihat alamat proxy yang sama
untuk semua browser lokal; rancangan trusted proxy tetap diperlukan sebelum produksi.
Backend memerlukan koneksi keluar ke Cloudflare untuk verifikasi CAPTCHA.

Dokumentasi endpoint, payload form, cookie, CSRF, serta alur curl/Postman:
[API.md](API.md). Semua contoh kosakata adalah placeholder buatan, bukan bahasa Lintang.

Turnstile memerlukan JavaScript ringan dari Cloudflare; seluruh alur form lainnya memakai SSR.
Kunci uji resmi Cloudflare menghasilkan hostname `example.com`, sehingga kiriman lokal dengan
origin `localhost` ditolak. Untuk menguji kiriman sukses, gunakan hostname uji yang cocok dan
kunci Turnstile pengembangan yang sesuai. Integrasi browser otomatis menggunakan verifier
pengganti hanya dalam test dan database sementara; aplikasi tidak punya bypass CAPTCHA lokal.
Jangan gunakan kunci uji pada produksi.

## Perintah

```sh
go test ./...
go vet ./...
go test -tags integration -count=1 ./internal/store ./internal/http
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go build ./cmd/app
go run ./cmd/app migrate up
go run ./cmd/app migrate down
```

`migrate down` hanya untuk database pengembangan yang boleh dihapus. Tes integrasi memakai
testcontainers dan selalu membuat database sementara, kemudian menguji migrasi naik/turun/naik.
Tidak ada variabel untuk mengarahkan tes ini ke database produksi.

Pada Docker Desktop Windows, bila deteksi socket testcontainers gagal, atur
`DOCKER_HOST=npipe:////./pipe/dockerDesktopLinuxEngine` dan
`TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`.

Target Makefile: `dev`, `build`, `test`, `test-int`, `lint`, `sqlc`, `sec`, `migrate-up`, `migrate-down`.
Typecheck merupakan bagian dari build Go. Tidak ada golangci-lint yang dikonfigurasi.

## Perilaku dan batasan

- Kiriman masuk `pending`; hanya transaksi moderasi yang membuat entri, ejaan, arti, dan contoh.
  Duplikat pending 24 jam diserialkan dengan advisory lock transaksi.
- `needs_fix` diperbaiki oleh tim lewat halaman tinjau, lalu dikembalikan ke antrean.
  Koreksi publik dan dukungan anonim merupakan fase berikutnya.
- Identitas akun dan hash sesi asal kiriman dicatat agar rotasi sesi tidak memungkinkan
  persetujuan sendiri. Kolom tersebut tidak diambil oleh query publik.
- Rate limit PostgreSQL per hash IP dan per hash ciri browser berlaku lintas proses/restart.
  Sidik browser berasal dari User-Agent dan Accept-Language; bukan identitas unik dan tetap dapat
  dipalsukan. Turnstile dan moderasi tetap diperlukan.
- Gangguan jaringan/5xx/internal-error CAPTCHA memakai batas terpisah yang lebih ketat.
  Token tidak valid, replay, hostname salah, dan salah konfigurasi ditolak.
- Hash kiriman, daftar blokir, serta data limiter dibersihkan otomatis saat startup dan tiap jam.
  Akun dinonaktifkan atau role berubah: semua sesi akun tersebut dicabut.
- Form tidak menerima `origin`, `relation`, visibility, atau status dari kiriman publik.
  Kolom asal/hubungan tetap memakai default skema; editor metadata dan penandaan sensitif
  merupakan P1. Filter sensitif/tersembunyi sudah berlaku sekarang pada semua query publik.
- Search memakai `simple` FTS dan trigram PostgreSQL. Normalisasi hanya mengubah kunci pencarian;
  ejaan asli disimpan. Aturan tambahan dapat diatur lewat `NORMALIZATION_REPLACEMENTS`.
  Perubahan aturan sesudah ada data memerlukan reindeks kolom normalized yang terencana.
- Header forwarding belum dipercaya. Reverse proxy perlu rancangan trusted proxy sebelum produksi,
  agar limiter tidak menganggap seluruh pengguna sebagai IP proxy yang sama.
- Tampilan publik dan tim memakai HTML server-rendered, CSS Tailwind statis, serta navigasi mobile ringan.
- API/ekspor publik, audio, dukungan, revisi, dan dasbor merupakan fase berikutnya.
  Endpoint yang belum diimplementasikan mengembalikan 404.

## Keputusan sebelum rilis

Lisensi dan teks kebijakan, kategori kata sensitif, tim verifikator, serta hosting masih perlu
diputuskan pengelola sesuai PRD §13. Tidak ada seed kosakata Lintang. Seluruh fixture adalah
placeholder buatan yang ditandai sebagai data uji.

Cadangan harian lintas lokasi, penyimpanan terenkripsi, serta uji pulih RPO/RTO belum diotomasi
karena target infrastruktur belum dipilih. Rilis MVP juga memerlukan uji penerimaan penutur,
konfigurasi HTTPS/ingress, dan pengukuran beban pada lingkungan tujuan.

Lihat `docs/IMPLEMENTASI_BACKEND.md` untuk keputusan teknis, versi, dan hasil validasi.
