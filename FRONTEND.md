# Frontend Kamus Lintang

Implementasi SSR sesuai referensi `docs/fe.html`, `docs/fe.pdf`, dan screenshot
header, tentang, kiriman, kamus, serta footer. Kebutuhan: FR-01–08, FR-20–23,
FR-30–31; invarian moderasi dan privasi tetap berlaku.

## Halaman dan perilaku

- `/`: hero pencarian, informasi proyek, target yang dilabeli, form kiriman, dan
  maksimal 20 entri publik. Tautan daftar lengkap menuju `/daftar`.
- `/cari`, `/daftar`, `/daftar/{huruf}`: pencarian dua arah, alfabet, hasil kosong,
  dan paginasi server. `/kata/{slug}` memuat detail kata terverifikasi.
- `/kirim`: POST nyata dengan CAPTCHA, isian tetap tersimpan saat validasi gagal,
  dan konfirmasi yang menyebut tinjauan manusia.
- `/admin/login`, `/admin/undangan`, `/admin/antrean`, `/admin/kiriman/{id}`,
  `/admin/pengguna`: halaman tim dengan sesi dan CSRF backend yang sama.

Tampilan contoh tidak menjadi seed. Tidak ada kosakata Lintang, jumlah aktual,
label asal kata, atau hubungan bahasa yang dikarang. Atribusi hanya dari consent.
Tentang tampil sebagai bagian beranda sesuai referensi; bukan kebijakan adat/lisensi.

Navigasi mobile dapat dibuka/tutup dan ditutup dengan Escape. Tanpa JavaScript
aplikasi, tautan navigasi tetap terlihat dan form pencarian tetap bekerja.
Turnstile tetap membutuhkan JavaScript dan jaringan Cloudflare.

## Build

CSS hasil build disertakan dalam Git dan di-embed bersama font serta JavaScript
lokal ke binary Go. Docker/backend tidak memerlukan Node saat runtime.

```sh
npm ci
npm run css
go build ./cmd/app
```

`make css` juga tersedia. Edit `web/styles/app.css`, lalu build ulang; jangan
mengedit `web/static/app.css` secara manual. Template: `web/templates/page.html`.
Node hanya untuk Tailwind CLI dan tes Playwright (dependensi dev, versi dipatok).
Tidak ada framework JS berat atau font CDN. Lisensi font ada di `web/static/fonts/`.

## Pengujian browser tanpa Postman

Pengujian berikut menyalakan server HTTPS dan database PostgreSQL **sementara**,
membuka browser headless, menjalankan alur publik/tim, menyimpan screenshot, lalu
menutup server dan membersihkan container. Tidak memakai database Compose lokal.
CAPTCHA double hanya berada di test Go; tidak tersedia pada binary aplikasi.

Prasyarat: Go, Node, Docker aktif, `npm ci`, Microsoft Edge. Untuk Chrome set
`UI_BROWSER=chrome`. Dari PowerShell:

```powershell
$env:UI_SMOKE = '1'
$env:DOCKER_HOST = 'npipe:////./pipe/dockerDesktopLinuxEngine'
$env:TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE = '/var/run/docker.sock'
go test -tags integration -run TestFrontendBrowser -count=1 -v ./internal/http
```

Dari shell Linux/macOS dengan Chrome tersedia:

```sh
UI_SMOKE=1 UI_BROWSER=chrome go test -tags integration -run TestFrontendBrowser -count=1 -v ./internal/http
```

Screenshot ada di `.impeccable/review/`: desktop, mobile, menu, pencarian, detail,
hasil kosong, galat kiriman, sukses, login, antrean, moderasi dan pengguna.
Semua kata pada screenshot pengujian adalah placeholder buatan, bukan bahasa Lintang.
Screenshot diabaikan Git; bisa dihasilkan ulang dengan perintah di atas.

Pengujian backend: `go test ./...`, `go vet ./...`, serta
`go test -tags integration -count=1 ./internal/store ./internal/http`.

## Menjalankan aplikasi untuk penggunaan manual

Jalankan backend dengan PostgreSQL, HTTPS dan variabel konfigurasi seperti pada
README. UI dilayani oleh binary yang sama; tidak ada server frontend terpisah.
Pada penyelesaian tugas ini stack Compose dan seluruh volumenya sudah dihapus
sesuai permintaan. Menyalakan Compose lagi membuat database kosong; sertifikat
lokal perlu disalin ulang karena volume CA lama juga telah dihapus.

Keterbatasan yang telah diketahui: kunci uji Cloudflare saat diperiksa mengembalikan
hostname `example.com`, sehingga konfigurasi `localhost` menolak kiriman (422).
Untuk penggunaan manual, gunakan pasangan kunci Turnstile yang terdaftar untuk
hostname aplikasi. Belum ada persetujuan mengubah mode anti-spam lokal; frontend
tidak menambahkan bypass. Tes browser di atas memverifikasi integrasi UI/backend,
bukan keberhasilan layanan CAPTCHA Cloudflare nyata.

Header Referrer-Policy memakai `same-origin`: form browser tetap membawa Origin
yang dapat diverifikasi, tanpa mengirim referrer ke layanan eksternal. CSP tetap
membatasi skrip/style/font ke sumber lokal, ditambah Turnstile sesuai kebutuhan.
