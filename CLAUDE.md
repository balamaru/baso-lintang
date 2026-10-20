@AGENTS.md

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
