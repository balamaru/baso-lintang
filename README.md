# Backend Kamus Bahasa Lintang

Implementasi MVP sesuai `docs/PRD.md`: FR-01–08, FR-20–23, FR-30–31, BR-1/3/4/7/8.
Monolit Go, chi, SSR `html/template`, PostgreSQL, pgx, query hasil sqlc, migrasi goose.
Belum merupakan persetujuan untuk rilis publik.

## Menjalankan lokal

Prasyarat: Go 1.27.3, Docker (untuk PostgreSQL dan tes integrasi), reverse proxy HTTPS lokal.

1. Salin `.env.example` menjadi `.env`. Isi rahasia acak minimal 32 karakter, password database,
   dan konfigurasi Turnstile. `.env` tidak dimuat otomatis oleh binary; ekspor variabel melalui shell
   atau gunakan Docker Compose.
2. Jalankan `docker compose --env-file .env -f deploy/docker-compose.yml up --build`.
   Migrasi berjalan sebelum aplikasi. Database dan port aplikasi hanya dipublikasikan ke loopback.
3. Tempatkan reverse proxy HTTPS lokal di depan `127.0.0.1:8080`; samakan origin dengan `BASE_URL`.
   Cookie selalu `Secure`, termasuk pada pengembangan. Endpoint operasional hanya di listener
   `OPS_ADDR` (default `127.0.0.1:9090`), tidak dipublikasikan Compose.
4. Buat admin awal dengan `app admin create --email <email> --name <nama>`, dengan `DATABASE_URL`
   dan `ADMIN_PASSWORD` pada environment. Pada Compose:
   `docker compose --env-file .env -f deploy/docker-compose.yml run --rm app admin create --email <email> --name <nama>`.
   Kosongkan `ADMIN_PASSWORD` setelah pemakaian.
5. Buka `/admin/login`; admin membuat undangan dari `/admin/pengguna`. Tautan ditampilkan
   hanya pada respons pembuatan undangan; tidak ada pengiriman email otomatis.

Turnstile memerlukan JavaScript ringan dari Cloudflare; seluruh alur form lainnya memakai SSR.
Gunakan kunci uji resmi Cloudflare pada pengembangan, lalu cocokkan hostname respons verifikasi
dengan hostname `BASE_URL`. Jangan gunakan kunci uji pada produksi.

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
- Tampilan adalah form SSR dasar untuk menguji backend. Integrasi visual Tailwind/htmx belum dikerjakan.
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
