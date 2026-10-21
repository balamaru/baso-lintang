# Alat pemeriksaan keamanan

Modul ini terpisah dari dependensi aplikasi. `gosec` 2.29.0 membawa `x/tools` 0.49.0,
yang belum dapat membaca export data versi 5 dari Go 1.27. Modul ini mematok
`x/tools` 0.52.0 agar pemeriksaan SSA berjalan.

Jalankan dari root proyek:

```sh
go run -modfile=tools/security/go.mod github.com/securego/gosec/v2/cmd/gosec -exclude-generated ./cmd/... ./internal/... ./db/... ./web/...
```

Pada PowerShell, beri tanda kutip pada `'-modfile=tools/security/go.mod'`.
Tidak memakai fitur AI/cloud gosec; pemeriksaan statis berjalan lokal.
Satu pengecualian `G101` dijelaskan langsung di kode untuk hash dummy pembanding
waktu login, yang bukan kredensial akun.
