# Kamus Lintang

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Penutur yang menyumbang melalui HP, termasuk lanjut usia; pencari kata; admin dan
verifikator tepercaya. Publik mencari dan mengirim tanpa akun. Tim memakai akun undangan.

## Product Purpose

Mendokumentasikan kosakata melalui kontribusi penutur dan verifikasi manusia.
Sumber utama tetap docs/PRD.md, docs/SYSTEM_DESIGN.md dan docs/DOKUMEN_PENDUKUNG.md.

## Capabilities and Constraints

Fokus P0: pencarian dua arah, daftar kata, detail, kiriman, moderasi dan akun tim.
SSR Go html/template, CSS Tailwind statis, JavaScript ringan. Ejaan tidak dibakukan.
Kiriman pending tidak tampil publik. Kontak privat; nama hanya dengan persetujuan.
Entri tersembunyi/sensitif tidak ditampilkan. Tidak mengarang kosakata atau klaim budaya.
Lisensi, kebijakan adat, hosting dan kesiapan produksi masih keputusan terbuka.

## Brand Commitments

Bahasa antarmuka Indonesia. Pengguna meminta implementasi berdasarkan docs/fe.html,
docs/fe.pdf dan docs/*.png; contoh visual bukan sumber data kamus resmi.

## Accessibility & Inclusion

HP dulu, keyboard, label form eksplisit, teks terbaca, kontras memadai, gerakan dapat
dikurangi. Fungsi utama tetap dapat digunakan tanpa JavaScript aplikasi.
