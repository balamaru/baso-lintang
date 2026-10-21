# Dokumentasi HTTP backend MVP

Mencakup FR-01–08, FR-20–23, FR-30–31 dan aturan BR-1/3/4/7/8.
Base URL Compose lokal: `https://localhost:8443`. Backend saat ini berupa SSR:
GET menghasilkan HTML; POST menerima `application/x-www-form-urlencoded`.
Payload JSON tidak didukung. API JSON `/api/v1/entries` belum tersedia (404).
File ini mendokumentasikan perilaku aktual, bukan kontrak fitur fase berikutnya.

## Menyiapkan klien

Ikuti [README](README.md) untuk menjalankan Compose, membuat admin dan menyalin CA lokal.
Contoh di bawah memakai Bash (Linux/macOS/Git Bash), curl dan Python 3. Pada PowerShell,
gunakan `curl.exe`, ganti sintaks variabel/kelanjutan baris, atau gunakan Postman.
Cookie jar dan halaman login disimpan di `.cache` yang diabaikan Git.

```sh
BASE=https://localhost:8443
CA=.cache/local-ca.crt
JAR=.cache/api-cookies.txt
csrf() { python -c 'import re,sys; print(re.search(r"name=\"csrf\" value=\"([^\"]+)\"", sys.stdin.read()).group(1))'; }
curl --cacert "$CA" -i "$BASE/"
```

Untuk Postman: impor CA lokal pada pengaturan sertifikat; aktifkan cookie jar.
Body POST: pilih **x-www-form-urlencoded**, tambahkan key/value seperti contoh di bawah.
Ambil nilai hidden `csrf` dari HTML GET, lalu kirim bersama cookie dari GET tersebut.
Matikan follow redirects saat ingin memeriksa status 303. Tidak memakai Bearer token.
`Origin`, bila dikirim, harus persis sama dengan base URL (tanpa slash akhir).

## Endpoint publik

| Method | Path | Parameter / hasil |
| --- | --- | --- |
| GET | `/` atau `/cari` | `q`, `arah=semua\|lintang\|indonesia`, `halaman` mulai 1; HTML, 20 hasil per halaman |
| GET | `/daftar` atau `/daftar/{huruf}` | Awalan ejaan, `halaman`; HTML daftar |
| GET | `/kata/{slug}` | Gunakan slug dari tautan hasil pencarian; 404 bila tidak tersedia/tersembunyi/sensitif |
| GET | `/kirim` | Form dengan widget Turnstile |
| POST | `/kirim` | Form kiriman, 200 bila diterima ke antrean atau duplikat pending |

```sh
curl --cacert "$CA" -G "$BASE/cari" --data-urlencode 'q=kata-uji-1' --data 'arah=lintang' --data 'halaman=1'
curl --cacert "$CA" "$BASE/daftar/k"
```

### Payload kiriman

Semua kata/contoh di dokumen ini **placeholder buatan, bukan kosakata Lintang**.

| Field | Wajib | Batas / nilai |
| --- | --- | --- |
| `headword` | Ya | 1–60 karakter; bukan URL/email; tanpa karakter kontrol |
| `gloss` | Ya | Arti Indonesia, 1–1.000 karakter |
| `explanation` | Tidak | Maks. 1.000 karakter |
| `pos` | Tidak | Jenis kata, maks. 60 karakter |
| `example_lintang`, `example_id` | Tidak | Masing-masing maks. 1.000 karakter |
| `name` | Tidak | Nama panggilan maks. 100 karakter |
| `contact` | Tidak | Maks. 254 karakter; tidak tampil publik |
| `consent` | Tidak | `on` untuk menyetujui atribusi nama; hilangkan jika tidak setuju |
| `website` | Tidak | Honeypot: harus kosong |
| `cf-turnstile-response` | Ya | Token widget; dummy berikut hanya untuk pasangan kunci uji lokal |

Seluruh field menolak karakter kontrol (termasuk newline). `origin`, `relation`,
`visibility`, dan `status` bukan field kiriman publik. Kiriman selalu masuk `pending`.

```sh
# Sengaja tanpa cookie login: kiriman anonim, dapat ditinjau admin uji.
curl --cacert "$CA" -i "$BASE/kirim" \
  -H "Origin: $BASE" \
  --data-urlencode 'headword=kata-uji-1' \
  --data-urlencode 'gloss=arti buatan untuk pengujian' \
  --data-urlencode 'explanation=Placeholder pengujian, bukan kosakata Lintang.' \
  --data-urlencode 'pos=nomina' \
  --data-urlencode 'example_lintang=contoh-uji-1' \
  --data-urlencode 'example_id=Contoh buatan untuk pengujian.' \
  --data-urlencode 'name=Penyumbang uji' \
  --data-urlencode 'contact=penyumbang@example.invalid' \
  --data 'consent=on' --data 'website=' \
  --data 'cf-turnstile-response=XXXX.DUMMY.TOKEN.XXXX'
```

Respons sukses: HTML berisi “Kiriman Anda akan ditinjau”; belum ada entri di pencarian.
Duplikat ejaan+arti pending dalam 24 jam: HTTP 200 dengan pesan “sudah diterima”.
Token dummy berasal dari [dokumentasi resmi Cloudflare](https://developers.cloudflare.com/turnstile/troubleshooting/testing/).
Cloudflare mengembalikan hostname `example.com` untuk kunci uji itu, jadi Compose lokal
(`localhost`) akan menolaknya. Untuk menguji kiriman sukses, siapkan hostname dan pasangan
kunci Turnstile pengembangan yang cocok. Jangan menonaktifkan validasi.

## Login tim dan CSRF

Semua POST `/admin/*`, termasuk login/undangan/logout, memerlukan token CSRF.
Cookie sesi `__Host-lintang-session` memakai Secure, HttpOnly, SameSite=Lax.
Token CSRF sebelum login terikat cookie `__Host-lintang-csrf`; sesudah login ambil
token baru dari form admin karena sesi dirotasi.

```sh
curl --cacert "$CA" -c "$JAR" "$BASE/admin/login" -o .cache/login.html
CSRF=$(csrf < .cache/login.html)
read -r -s -p 'Password admin dari .env: ' PASSWORD; printf '\n'
# Password dikirim via stdin agar tidak muncul dalam argumen proses curl.
printf '%s' "$PASSWORD" | curl --cacert "$CA" -i -b "$JAR" -c "$JAR" "$BASE/admin/login" \
  -H "Origin: $BASE" --data-urlencode "csrf=$CSRF" \
  --data-urlencode 'email=admin@example.invalid' --data-urlencode 'password@-'
unset PASSWORD
```

Sukses: 303 ke `/admin/antrean`. Gagal: 401 pesan generik; setelah 5 kegagalan akun
terkunci 15 menit. Limiter login per IP juga dapat menghasilkan 429.

## Antrean dan moderasi

```sh
curl --cacert "$CA" -b "$JAR" -G "$BASE/admin/antrean" \
  --data 'status=pending' --data 'urutan=lama' --data-urlencode 'q=kata-uji-1'
# Ambil ID dari tautan /admin/kiriman/{id} pada HTML antrean; jangan menebaknya.
ID=1 # Ganti dengan ID sebenarnya.
curl --cacert "$CA" -b "$JAR" "$BASE/admin/kiriman/$ID" -o .cache/review.html
CSRF=$(csrf < .cache/review.html)
curl --cacert "$CA" -i -b "$JAR" "$BASE/admin/kiriman/$ID" \
  --data-urlencode "csrf=$CSRF" --data 'action=approve' \
  --data-urlencode 'headword=kata-uji-1' \
  --data-urlencode 'gloss=arti buatan untuk pengujian' \
  --data-urlencode 'explanation=Placeholder pengujian, bukan kosakata Lintang.' \
  --data-urlencode 'variants=varian-uji-1' --data-urlencode 'note=Ditinjau untuk pengujian lokal.'
```

Sukses: 303 ke antrean. Cari kembali lewat `/cari`: kata yang diterima kini tampil.
Kiriman dari akun/sesi reviewer sendiri tidak boleh disetujui atau digabung (403).

GET `/admin/antrean`: `status=pending|approved|rejected|needs_fix`, `q`,
`urutan=baru` (selain itu terlama), `halaman`. GET/POST `/admin/kiriman/{id}`
tersedia untuk admin dan verifier. Payload POST:

| `action` | Field tambahan | Transisi |
| --- | --- | --- |
| `approve` | `headword`, `gloss`, field konten opsional; `variants` dipisah koma (maks. 20) | pending → approved + entri baru |
| `merge` | Konten seperti approve dan `target` ID entri yang ada | pending → approved + arti/varian pada target |
| `reject` | `note` wajib | pending → rejected |
| `needs_fix` | `note` wajib | pending → needs_fix |
| `resubmit` | `headword`, `gloss`, konten perbaikan | needs_fix → pending |

`note` maksimal 4.000 byte pada backend (UI membatasi 1.000 karakter). Semua action
memerlukan `csrf`; konten approve/merge/resubmit mengikuti batas kiriman.
Mengulangi keputusan terhadap kiriman yang sudah ditinjau menghasilkan 409.

Contoh payload lain (Body Postman x-www-form-urlencoded, bukan JSON):

```text
csrf=<token dari GET tinjau>&action=reject&note=Data%20uji%20ditolak
csrf=<token dari GET tinjau>&action=needs_fix&note=Lengkapi%20arti
csrf=<token dari GET tinjau>&action=resubmit&headword=kata-uji-2&gloss=Arti%20uji
csrf=<token dari GET tinjau>&action=merge&target=<ID entri>&headword=varian-uji-2&gloss=Arti%20uji
```

## Akun dan undangan

GET `/admin/pengguna` hanya admin. Verifier mendapat 403. Ambil CSRF dari HTML:

```sh
curl --cacert "$CA" -b "$JAR" "$BASE/admin/pengguna" -o .cache/users.html
CSRF=$(csrf < .cache/users.html)
curl --cacert "$CA" -b "$JAR" "$BASE/admin/pengguna" \
  --data-urlencode "csrf=$CSRF" --data 'action=invite' \
  --data-urlencode 'email=verifier@example.invalid' --data 'role=verifier'
```

Respons 200 menampilkan tautan sekali pakai (72 jam); tidak ada email otomatis.
Jaga tautan/token ini sebagai rahasia. Buka tautan di cookie jar/browser terpisah.
GET `/admin/undangan?token=...` memberi form dan cookie CSRF; POST `/admin/undangan`
memerlukan `csrf`, `token`, `name` (1–100 byte), `password` (minimal 12 karakter, maksimal 256 byte).
Sukses: 303 ke login; token tidak valid/kedaluwarsa/terpakai atau isian salah: 422.

POST `/admin/pengguna` untuk ubah akun:

```text
csrf=<token>&action=update&id=<ID akun>&role=verifier&active=on
```

Role sah: `admin` atau `verifier`. Hilangkan `active` untuk menonaktifkan.
Perubahan role/status mencabut sesi akun. Admin tidak dapat memperbarui akunnya sendiri
melalui endpoint ini. Sukses 200. ID didapat dari form daftar pengguna.

Logout: POST `/admin/logout` dengan `csrf` dari form admin dan cookie sesi; sukses
303 ke login. Tidak tersedia pendaftaran publik.

## Status dan kasus negatif

| Status | Makna |
| --- | --- |
| 200 | HTML berhasil ditampilkan; kiriman masuk antrean/duplikat; pengelolaan akun sukses |
| 303 | Redirect sesudah login/logout/moderasi/aktivasi undangan; akses tim tanpa sesi menuju login |
| 400 / 414 | Form terlalu besar/tidak valid (batas body 32 KiB), atau query string >4.096 byte |
| 401 | Login gagal/terkunci |
| 403 | CSRF/Origin salah, role tidak cukup, atau persetujuan sendiri |
| 404 | Data/rute tidak ditemukan, termasuk entri tersembunyi/sensitif |
| 409 | Transisi kiriman tidak dapat diulang |
| 422 | Validasi isian/CAPTCHA/undangan gagal; form kiriman tetap terisi |
| 429 | Batas kiriman/login/undangan tercapai |
| 500 | Kesalahan internal dengan pesan generik |

Uji minimal: pencarian belum memuat kiriman pending; token CAPTCHA kosong → 422;
honeypot berisi → 422 setelah CAPTCHA; POST admin tanpa CSRF → 403;
verifier membuka pengguna → 403; kiriman sendiri disetujui → 403;
approve dua kali → 409. Batas kiriman awal 10/jam dan 30/hari; seluruh pengunjung
Compose berbagi alamat proxy. Jalankan kasus yang memakai kuota secara terencana.

## Endpoint operasional

Hanya listener internal `127.0.0.1:9090` dalam container app, tidak lewat Caddy:

```sh
docker compose --env-file .env -f deploy/docker-compose.yml exec app wget -S -O - http://127.0.0.1:9090/readyz
```

GET `/healthz`: 204; GET `/readyz`: 204 jika PostgreSQL tersedia, 503 jika tidak;
GET `/metrics`: 200 teks Prometheus berisi penghitung request/error. Gunakan listener
ini hanya untuk operasi internal. Pengujian integrasi otomatis: `make test-int`.
