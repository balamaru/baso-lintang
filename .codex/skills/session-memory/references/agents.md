# Integrasi per coding agent

Semua agent memakai data dan skrip yang sama (`.agent-memory/`). Yang berbeda hanya cara
agent "diingatkan" untuk memuat dan memperbarui memori. `init` mengurus sebagian besar.
`M` = `python .agent-memory/bin/memory.py` (atau `python3`).

## Claude Code (otomatis penuh lewat hook)
- Pasang skill: salin folder `session-memory/` ke `~/.claude/skills/` (semua proyek) atau
  `<proyek>/.claude/skills/` (satu proyek).
- `init` menulis blok bootstrap ke `CLAUDE.md` dan menggabungkan hook ke `.claude/settings.json`:
  - `SessionStart` → `M start --agent claude-code --hook`. Output hook ini masuk ke konteks,
    jadi briefing sudah ada sebelum prompt pertama dikerjakan. Untuk `source` = `resume` /
    `compact`, session yang sama dilanjutkan; untuk `startup` / `clear`, session baru.
  - `Stop` dan `PreCompact` → `M checkpoint --quiet` (merekam file berubah; murah, tanpa output).
  - `SessionEnd` → `M end --auto` (menutup session dengan ringkasan otomatis bila agent belum
    menjalankan `end --summary`).
- Hook dijalankan dari direktori proyek. Bila `python3`/`python` tidak ada di PATH, ubah
  perintahnya di `.claude/settings.json` (mis. `py -3` di Windows), atau ulangi
  `init --python "py -3"`.
- Hook hanya merekam hal mekanis; ringkasan bermakna tetap dari `log`/`end --summary` agent.

## OpenAI Codex (CLI / IDE)
- Codex membaca `AGENTS.md` di root proyek; `init` selalu menulis blok bootstrap di sana,
  sehingga Codex menjalankan `M start --agent codex` di awal dan `M end` di akhir.
- Bila versi Codex Anda mendukung Agent Skills, salin folder skill ke direktori skills Codex
  (mis. `~/.codex/skills/`); bila tidak, `AGENTS.md` + `.agent-memory/PROTOCOL.md` sudah cukup.
- Codex tidak memiliki hook SessionEnd; session yang lupa ditutup ditutup otomatis oleh
  `start` berikutnya.

## Gemini CLI
- `init --agents auto` menulis ke `GEMINI.md` bila file itu ada; paksa dengan
  `init --agents agents,gemini`. Alternatif: atur Gemini CLI agar membaca `AGENTS.md`
  sebagai context file.

## Cursor
- `init` menulis `.cursor/rules/session-memory.mdc` (`alwaysApply: true`) bila folder
  `.cursor/` ada; paksa dengan `--agents agents,cursor`.

## GitHub Copilot (agent mode / coding agent)
- Ditulis ke `.github/copilot-instructions.md` bila file itu ada; paksa dengan
  `--agents agents,copilot`. Copilot coding agent juga membaca `AGENTS.md`.

## Windsurf / Cline
- `.windsurfrules` / `.clinerules` (bila sudah ada), atau paksa dengan
  `--agents agents,windsurf` / `--agents agents,cline`.

## Aider
- Muat memori sebagai konteks baca-saja: `aider --read .agent-memory/MEMORY.md --read .agent-memory/requirements.md`,
  atau tambahkan ke `.aider.conf.yml` (`read: [...]`).
- Jalankan perintah memori dari chat dengan `/run python .agent-memory/bin/memory.py ...`.

## Agent lain
Apa pun yang bisa membaca file instruksi proyek dan menjalankan perintah shell bisa
memakai protokol ini: arahkan agent ke `.agent-memory/PROTOCOL.md`.

## Handoff antar agent
Karena semua agent menulis ke tempat yang sama, alur seperti "Claude Code mengerjakan fitur,
lalu Codex melanjutkan test" bekerja tanpa langkah tambahan: Codex menjalankan `start`, melihat
ringkasan session Claude Code di blok "Session terakhir" dan TODO-nya, lalu melanjutkan.
Gunakan `--agent` yang berbeda per agent agar log tidak tercampur.

## Git
- Commit `.agent-memory/` (kecuali `state.json`, yang sudah di-gitignore) agar memori ikut
  ke anggota tim dan mesin lain. Bila memori bersifat pribadi, tambahkan `.agent-memory/`
  ke `.gitignore` proyek.
- Tanpa git, skrip tetap jalan; deteksi file berubah hanya berasal dari hash di peta kode.
