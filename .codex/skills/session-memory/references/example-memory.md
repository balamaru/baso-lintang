# Contoh memori yang terisi baik

Contoh dari proyek nyata (K8s Health Checker → Discord). Perhatikan: padat, faktual, menyebut
path dan nama env, menyimpan *alasan*, dan tidak memuat nilai rahasia apa pun.

## `.agent-memory/MEMORY.md`

```markdown
# Memori Proyek: kubernetes-health-checker

## Tujuan & sumber requirements
- Requirements awal: `docs/goals.md`
- Tujuan: memantau kesehatan cluster K8s (read-only) dan mengirim temuan + analisis ke Discord.

## Status saat ini
- v1.2.0: check `logs` (temuan `pod.log_errors`) untuk pod Running/Ready yang log-nya error. 37 test lulus.
- Belum pernah diuji di cluster nyata maupun dengan API LLM sungguhan.

## Keputusan penting
- 2 container: `checker` (polling K8s API read-only) + `notifier` (webhook HTTP → Discord embed).
- Python 3.12 stdlib saja, non-root, read-only rootfs — image kecil, tanpa CVE dependensi.
- Semua konfigurasi via env (ConfigMap+Secret). Auth checker→notifier: shared secret `WEBHOOK_AUTH_TOKEN`.
- State dedup/reminder/resolved di /data/state.json; temuan TIDAK di-resolve saat API down.
- Pola log berbasis level (ERROR/FATAL, level=error, klog E0000, panic:, Traceback), bukan kata "error" polos — menekan false positive.
- LLM tidak perlu model frontier (±3–5k token input → 1 JSON); default kelas Flash/Mini/Haiku.

## Konvensi & cara kerja
- Test: `python -m unittest discover tests`
- Rahasia disamarkan `redact.py` sebelum ke LLM/Discord; nilai env/Secret tidak pernah dibaca.
- Fitur baru → bagian "Yang baru di x.y" + "Upgrade" di README.

## Peta kode
- Daftar file penting + fungsinya: `memory.py files list`

## Langkah berikutnya
- Build image Docker; uji di cluster nyata dan dengan API LLM sungguhan.
- Tuning pola/ambang log dengan log aplikasi nyata (false positive per tim).

## Masalah terbuka / jebakan
- Pod yang sudah punya temuan lain di-skip oleh check `logs` (disengaja, hindari duplikasi).
- RBAC butuh `pods/log` (get) dan `storageclasses`; tanpa itu diagnostik diam-diam kosong.
```

## `.agent-memory/requirements.md` (potongan)

```markdown
Sumber: `docs/goals.md`
- [x] R1: Polling K8s API read-only untuk pod/node/deployment — checks.py, RBAC read-only
- [x] R2: Notifikasi Discord dengan dedup, reminder, resolved — notifier/, state.json
- [x] R3: Analisis akar masalah dengan LLM dari banyak provider — llm.py (6 provider)
- [x] R4: Deteksi pod Running/Ready yang log-nya error — logscan.py, tests/test_logscan.py
- [~] R5: Siap deploy di cluster nyata — manifest ada; image & uji cluster belum
- [ ] R6: Ambang/pola log teruji dengan log produksi
```

## Contoh baris log session yang baik
```
- 09:12 [TASK] tambah deteksi log error untuk pod Running/Ready
- 10:05 [DECISION] gabung temuan per workload+container (ReplicaSet→Deployment) agar 10 replika = 1 alert
- 10:40 [DONE] checks.check_logs + anotasi health-checker.io/log-scan=false
- 11:02 [FIX] signature log tidak stabil — normalisasi angka/IP/UUID/timestamp
- 11:30 [TODO] uji pola dengan log nginx & PHP nyata
```
