#!/usr/bin/env python3
"""
session-memory — memori lintas-session & lintas-agent untuk coding agent
(Claude Code, Codex, Gemini CLI, Cursor, Copilot, Aider, Cline, dll.).

Python 3.8+ stdlib saja. Semua data disimpan di <proyek>/.agent-memory/.
Jalankan `python memory.py -h` untuk daftar perintah.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

VERSION = "1.0.0"
MEM = ".agent-memory"
TAGS = {
    "task": "TASK", "done": "DONE", "decision": "DECISION", "todo": "TODO",
    "blocker": "BLOCKER", "fix": "FIX", "note": "NOTE",
}
REQ_STATUS = {"x": "x", "done": "x", "~": "~", "partial": "~", " ": " ", "o": " ",
              "todo": " ", "open": " ", "-": "-", "drop": "-", "dropped": "-"}
BRIEF_LIMIT_CHARS = 16000      # ~4k token; di atas ini briefing dipangkas
KEEP_SESSIONS = 8              # session detail yang disimpan; sisanya diringkas ke history.md
STALE_HOURS = 12               # session agent lain yang terbuka > ini ditutup otomatis
SKIP_DIRS = {".git", "node_modules", ".venv", "venv", "__pycache__", MEM, "dist",
             "build", ".next", "target", ".idea", ".vscode", "vendor", ".tox"}
ROOT = Path.cwd()

# --------------------------------------------------------------------------- util

def now() -> dt.datetime:
    return dt.datetime.now().replace(microsecond=0)


def ts(d: dt.datetime | None = None) -> str:
    return (d or now()).strftime("%Y-%m-%d %H:%M")


def git(args, cwd=None):
    try:
        r = subprocess.run(["git", *args], cwd=str(cwd or ROOT), capture_output=True,
                           text=True, encoding="utf-8", errors="replace", timeout=20)
        return r.stdout.strip() if r.returncode == 0 else None
    except Exception:
        return None


def find_root() -> Path:
    for key in ("SESSION_MEMORY_ROOT", "CLAUDE_PROJECT_DIR"):
        v = os.environ.get(key)
        if v and Path(v).is_dir():
            return Path(v).resolve()
    cur = Path.cwd().resolve()
    for p in (cur, *cur.parents):
        if (p / MEM).is_dir():
            return p
    top = git(["rev-parse", "--show-toplevel"], cwd=cur)
    return Path(top).resolve() if top else cur


def read(p: Path, default: str = "") -> str:
    try:
        return p.read_text(encoding="utf-8")
    except Exception:
        return default


def write(p: Path, text: str) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    tmp = p.with_suffix(p.suffix + ".tmp")
    tmp.write_text(text, encoding="utf-8", newline="\n")
    os.replace(tmp, p)


def sanitize(name: str) -> str:
    return re.sub(r"[^a-z0-9._-]+", "-", (name or "agent").lower()).strip("-") or "agent"


def tokens(text: str) -> int:
    return max(1, len(text) // 4)


# ----------------------------------------------------------------- redaksi rahasia
_SECRET = [
    (re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----", re.S),
     "[REDACTED_PRIVATE_KEY]"),
    (re.compile(r"\bsk-(?:ant-|proj-)?[A-Za-z0-9_\-]{16,}"), "[REDACTED_KEY]"),
    (re.compile(r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b"), "[REDACTED_AWS_KEY]"),
    (re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})"), "[REDACTED_TOKEN]"),
    (re.compile(r"\bglpat-[A-Za-z0-9_\-]{16,}"), "[REDACTED_TOKEN]"),
    (re.compile(r"\bxox[abprs]-[A-Za-z0-9-]{10,}"), "[REDACTED_TOKEN]"),
    (re.compile(r"\bAIza[0-9A-Za-z_\-]{30,}"), "[REDACTED_KEY]"),
    (re.compile(r"\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{8,}"), "[REDACTED_JWT]"),
    (re.compile(r"https://(?:ptb\.|canary\.)?discord(?:app)?\.com/api/webhooks/\S+"), "[REDACTED_WEBHOOK]"),
    (re.compile(r"https://hooks\.slack\.com/\S+"), "[REDACTED_WEBHOOK]"),
    (re.compile(r"(?i)\b(bearer)\s+[A-Za-z0-9._~+/\-]{16,}=*"), r"\1 [REDACTED]"),
    (re.compile(r"(?i)\b([a-z0-9_]*(?:password|passwd|pwd|secret|token|api[_-]?key|apikey|"
                r"access[_-]?key|private[_-]?key)[a-z0-9_]*)(\s*[:=]\s*)(['\"]?)[^\s'\"]{4,}\3"),
     r"\1\2[REDACTED]"),
    (re.compile(r"(?i)\b([a-z][a-z0-9+.-]*://)[^\s:/@]+:[^\s@/]+@"), r"\1[REDACTED]@"),
    (re.compile(r"\b[\w.+-]+@[A-Za-z0-9-]+\.[A-Za-z0-9.-]+\b"), "[REDACTED_EMAIL]"),
]


def redact(text: str) -> str:
    for rx, rep in _SECRET:
        text = rx.sub(rep, text)
    return text


# ----------------------------------------------------------------------- paths/state
class Paths:
    def __init__(self, root: Path):
        self.root = root
        self.mem = root / MEM
        self.memory = self.mem / "MEMORY.md"
        self.req = self.mem / "requirements.md"
        self.filemap = self.mem / "filemap.json"
        self.state = self.mem / "state.json"
        self.sessions = self.mem / "sessions"
        self.archive = self.sessions / "archive"
        self.history = self.mem / "history.md"
        self.bin = self.mem / "bin"
        self.protocol = self.mem / "PROTOCOL.md"


def load_state(p: Paths) -> dict:
    try:
        st = json.loads(read(p.state, "{}"))
    except Exception:
        st = {}
    st.setdefault("open", {})
    st.setdefault("last_session", None)
    st.setdefault("last_end_commit", None)
    return st


def save_state(p: Paths, st: dict) -> None:
    write(p.state, json.dumps(st, indent=2, ensure_ascii=False) + "\n")


def load_filemap(p: Paths) -> dict:
    try:
        return json.loads(read(p.filemap, "{}"))
    except Exception:
        return {}


def save_filemap(p: Paths, fm: dict) -> None:
    write(p.filemap, json.dumps(dict(sorted(fm.items())), indent=2, ensure_ascii=False) + "\n")


def file_hash(path: Path):
    try:
        return hashlib.sha1(path.read_bytes()).hexdigest()[:12]
    except Exception:
        return None


def rel(path: str) -> str:
    p = Path(path)
    if p.is_absolute():
        try:
            p = p.resolve().relative_to(ROOT)
        except ValueError:
            pass
    return p.as_posix().lstrip("./") if str(p) not in (".", "") else "."


# ------------------------------------------------------------------------------ git
def head():
    return git(["rev-parse", "--short", "HEAD"])


def branch():
    return git(["rev-parse", "--abbrev-ref", "HEAD"])


def is_git() -> bool:
    return git(["rev-parse", "--is-inside-work-tree"]) == "true"


def changed_files(base) -> dict:
    """File yang berubah sejak commit `base` (termasuk commit di antaranya + belum di-commit)."""
    out: dict = {}
    if not is_git():
        return out
    d = git(["diff", "--name-status", base]) if base else git(["diff", "--name-status", "HEAD"])
    for line in (d or "").splitlines():
        parts = line.split("\t")
        if len(parts) >= 2:
            out[parts[-1]] = parts[0][0]
    for path in (git(["ls-files", "--others", "--exclude-standard"]) or "").splitlines():
        out.setdefault(path, "A")
    return {k: v for k, v in out.items() if not k.startswith(MEM + "/")}


BOOT_FILES = {"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".windsurfrules", ".clinerules",
              ".claude/settings.json", ".claude/settings.local.json",
              ".cursor/rules/session-memory.mdc", ".github/copilot-instructions.md"}


def unmapped_new(files: dict, fm: dict) -> list:
    return [f for f, s in files.items() if s in ("A", "?") and f not in fm and f not in BOOT_FILES]


# ------------------------------------------------------------------------- templates
MEMORY_TEMPLATE = """# Memori Proyek: {name}

> Dikelola skill session-memory. Bagian manual diedit agent (ringkas, edit di tempat,
> jangan menumpuk); blok `auto` diperbarui skrip. Target < ~150 baris.

## Tujuan & sumber requirements
- Requirements awal: {req}
- Tujuan: (1–3 kalimat)

## Status saat ini
- (versi/milestone, apa yang sudah berjalan)

## Keputusan penting
- (keputusan — alasan singkat)

## Konvensi & cara kerja
- Build/test: (perintah)

## Peta kode
- Daftar file penting + fungsinya: `memory.py files list`

## Langkah berikutnya
- (isi)

## Masalah terbuka / jebakan
- (isi)

<!-- auto:last-session -->
<!-- /auto:last-session -->
"""

REQ_TEMPLATE = """# Requirements (checklist turunan)

Sumber: {src}
Dokumen sumber tidak diubah. Checklist ini diisi agent sekali dari sumber, lalu statusnya diperbarui.
Status: [ ] belum · [~] sebagian · [x] selesai · [-] dibatalkan
Format: - [ ] R1: <requirement singkat> — <bukti/catatan: file, test>

"""

BOOT_START, BOOT_END = "<!-- session-memory:start -->", "<!-- session-memory:end -->"


def bootstrap_block(py: str) -> str:
    cmd = f"{py} .agent-memory/bin/memory.py"
    return f"""{BOOT_START}
## Memori session (session-memory)
Proyek ini memakai `.agent-memory/` sebagai memori lintas session & lintas agent.
1. **Awal session, SEBELUM mengerjakan prompt apa pun**: jika konteks belum berisi
   `=== SESSION MEMORY`, jalankan `{cmd} start --agent <nama-agent>`. Cocokkan prompt
   dengan requirements & session terakhir di output-nya. Jangan membaca ulang seluruh kode:
   baca hanya file yang relevan dengan task dan file yang ditandai berubah.
2. **Selama session**: `{cmd} log <task|done|decision|todo|blocker|fix|note> "teks singkat"`
   setiap ada langkah berarti; `{cmd} req set R<n> <x|~|-> "bukti"` saat requirement maju;
   `{cmd} files add <path> "fungsi"` untuk file penting baru.
3. Perbarui bagian manual `.agent-memory/MEMORY.md` bila status/keputusan/langkah berikutnya berubah.
4. **Akhir session** (task tuntas / user pamit): `{cmd} end --summary "ringkasan 2–4 kalimat"`.
   Bila lebih dari satu agent aktif, tambahkan `--agent <nama-agent>` yang sama di `log`/`end`.
5. Jangan pernah menulis rahasia (API key, token, password) atau data pribadi ke memori.
Detail protokol: `.agent-memory/PROTOCOL.md` (gunakan `python3` bila `python` tidak ada).
{BOOT_END}"""


FALLBACK_PROTOCOL = """# Protokol session-memory
Lihat blok session-memory di AGENTS.md. Perintah: `python .agent-memory/bin/memory.py -h`.
"""


# ------------------------------------------------------------------------ init
REQ_NAME_RX = re.compile(r"(requirement|goal|prd|spec|brief|scope|kebutuhan|tujuan)s?[\w\-]*\.(md|txt|rst)$", re.I)


def detect_requirements(root: Path) -> list:
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")]
        depth = len(Path(dirpath).relative_to(root).parts)
        if depth > 3:
            dirnames[:] = []
            continue
        for f in filenames:
            if f.lower() == "requirements.txt":   # pip, bukan requirements produk
                continue
            if REQ_NAME_RX.search(f):
                found.append((Path(dirpath) / f).relative_to(root).as_posix())
    return sorted(found, key=lambda s: (s.count("/"), s))[:10]


def upsert_block(path: Path, block: str, header: str = "") -> str:
    text = read(path)
    if BOOT_START in text and BOOT_END in text:
        new = re.sub(re.escape(BOOT_START) + r".*?" + re.escape(BOOT_END), lambda m: block, text, flags=re.S)
        action = "diperbarui"
    else:
        new = (text.rstrip() + "\n\n" if text.strip() else header) + block + "\n"
        action = "ditambahkan" if text.strip() else "dibuat"
    if new != text:
        write(path, new)
    return action


def install_claude_hooks(root: Path, py: str) -> str:
    settings = root / ".claude" / "settings.json"
    try:
        data = json.loads(read(settings, "{}") or "{}")
    except Exception:
        return "LEWATI: .claude/settings.json bukan JSON valid — pasang hook manual (lihat references/agents.md)"
    cmd = f"{py} .agent-memory/bin/memory.py"
    wanted = {
        "SessionStart": f"{cmd} start --agent claude-code --hook",
        "Stop": f"{cmd} checkpoint --agent claude-code --quiet",
        "PreCompact": f"{cmd} checkpoint --agent claude-code --quiet",
        "SessionEnd": f"{cmd} end --agent claude-code --auto",
    }
    hooks = data.setdefault("hooks", {})
    for event, command in wanted.items():
        groups = hooks.setdefault(event, [])
        exists = any("memory.py" in h.get("command", "") for g in groups for h in g.get("hooks", []))
        if not exists:
            groups.append({"hooks": [{"type": "command", "command": command, "timeout": 30}]})
    write(settings, json.dumps(data, indent=2, ensure_ascii=False) + "\n")
    return "hook Claude Code dipasang di .claude/settings.json (SessionStart/Stop/PreCompact/SessionEnd)"


def cmd_init(a):
    p = Paths(ROOT)
    for d in (p.mem, p.sessions, p.archive, p.bin):
        d.mkdir(parents=True, exist_ok=True)
    py = a.python or ("python" if os.name == "nt" else "python3")
    me = Path(__file__).resolve()
    if me != (p.bin / "memory.py").resolve():
        shutil.copy2(me, p.bin / "memory.py")
    proto_src = me.parent.parent / "references" / "protocol.md"
    if proto_src.exists():
        shutil.copy2(proto_src, p.protocol)
    elif not p.protocol.exists():
        write(p.protocol, FALLBACK_PROTOCOL)
    write(p.mem / ".gitignore", "state.json\n*.tmp\n")

    reqs = [rel(a.req)] if a.req else detect_requirements(ROOT)
    src = ", ".join(f"`{r}`" for r in reqs) if reqs else "(belum ditemukan — tanyakan user / isi manual)"
    if not p.req.exists():
        write(p.req, REQ_TEMPLATE.format(src=src))
    if not p.memory.exists():
        write(p.memory, MEMORY_TEMPLATE.format(name=ROOT.name, req=src))
    if not p.filemap.exists():
        save_filemap(p, {})

    agents = {s.strip().lower() for s in (a.agents or "auto").split(",") if s.strip()}
    auto = "auto" in agents or "all" in agents
    block = bootstrap_block(py)
    report = []
    targets = [("agents", ROOT / "AGENTS.md", True), ("claude", ROOT / "CLAUDE.md", True),
               ("gemini", ROOT / "GEMINI.md", (ROOT / "GEMINI.md").exists()),
               ("copilot", ROOT / ".github" / "copilot-instructions.md",
                (ROOT / ".github" / "copilot-instructions.md").exists()),
               ("windsurf", ROOT / ".windsurfrules", (ROOT / ".windsurfrules").exists()),
               ("cline", ROOT / ".clinerules", (ROOT / ".clinerules").is_file())]
    for key, path, auto_on in targets:
        if key in agents or "all" in agents or (auto and auto_on) or (key == "agents"):
            report.append(f"{path.relative_to(ROOT).as_posix()}: {upsert_block(path, block)}")
    if "cursor" in agents or "all" in agents or (auto and (ROOT / ".cursor").exists()):
        cpath = ROOT / ".cursor" / "rules" / "session-memory.mdc"
        hdr = "---\ndescription: Memori session lintas agent (session-memory)\nalwaysApply: true\n---\n"
        report.append(f".cursor/rules/session-memory.mdc: {upsert_block(cpath, block, hdr)}")
    if not a.no_hooks and (auto or "claude" in agents):
        report.append(install_claude_hooks(ROOT, py))

    print(f"session-memory {VERSION} siap di {MEM}/")
    for r in report:
        print(f"  - {r}")
    print(f"  - sumber requirements terdeteksi: {src}")
    print("\nLANGKAH AGENT BERIKUTNYA (sekali saja):")
    print("  1. Baca dokumen requirements sumber SEKALI, lalu isi .agent-memory/requirements.md "
          "(satu baris per requirement, beri status jika sudah jelas selesai).")
    print("  2. Isi bagian manual .agent-memory/MEMORY.md (tujuan, status, keputusan, konvensi). "
          "Jika ada catatan lama (notes/CLAUDE.md/README), ringkas ke sana.")
    print(f"  3. Daftarkan 10–40 file inti: {py} .agent-memory/bin/memory.py files add <path> \"fungsi\"")
    print(f"  4. Mulai session: {py} .agent-memory/bin/memory.py start --agent <nama>")


# ----------------------------------------------------------------------- sessions
LOG_RX = re.compile(r"^- (\d\d:\d\d) \[([A-Z]+)\] (.*)$")


def session_path(p: Paths, sid: str) -> Path:
    return p.sessions / f"{sid}.md"


def new_session(p: Paths, st: dict, agent: str) -> str:
    base = f"{now():%Y-%m-%d_%H%M}_{agent}"
    sid, n = base, 2
    while session_path(p, sid).exists():
        sid, n = f"{base}-{n}", n + 1
    c, b = head(), branch()
    write(session_path(p, sid),
          f"# Session {sid}\n- agent: {agent}\n- mulai: {ts()}\n"
          f"- git: {c or '-'} ({b or 'tanpa git'})\n- status: open\n\n## Log\n")
    st["open"][agent] = {"id": sid, "start": ts(), "start_commit": c, "files": {}}
    return sid


def parse_logs(text: str) -> list:
    return [m.groups() for m in (LOG_RX.match(l) for l in text.splitlines()) if m]


def auto_summary(logs, files) -> str:
    done = [t for _, tag, t in logs if tag in ("DONE", "FIX")]
    tasks = [t for _, tag, t in logs if tag == "TASK"]
    parts = ["(otomatis — agent tidak menulis ringkasan)"]
    if tasks:
        parts.append("Task: " + "; ".join(tasks[:3]) + ".")
    if done:
        parts.append(f"{len(done)} langkah selesai, a.l.: " + "; ".join(done[-4:]) + ".")
    parts.append(f"{len(files)} file berubah.")
    if not logs:
        parts.append("Tidak ada log — periksa diff git untuk detail.")
    return " ".join(parts)


def update_memory_auto(p: Paths, block: str) -> None:
    text = read(p.memory) or MEMORY_TEMPLATE.format(name=ROOT.name, req="-")
    start, end = "<!-- auto:last-session -->", "<!-- /auto:last-session -->"
    full = f"{start}\n{block}\n{end}"
    if start in text and end in text:
        text = re.sub(re.escape(start) + r".*?" + re.escape(end), lambda m: full, text, flags=re.S)
    else:
        text = text.rstrip() + "\n\n" + full + "\n"
    write(p.memory, text)


def close_session(p: Paths, st: dict, agent: str, summary=None, reason=None) -> dict | None:
    info = st["open"].pop(agent, None)
    if not info:
        return None
    sid = info["id"]
    sp = session_path(p, sid)
    text = read(sp, f"# Session {sid}\n\n## Log\n")
    files = dict(info.get("files") or {})
    files.update(changed_files(info.get("start_commit")))
    logs = parse_logs(text)
    is_auto = not summary
    summary = redact(summary) if summary else auto_summary(logs, files)
    status = "closed-auto" if is_auto else "closed"
    text = text.replace("- status: open", f"- status: {status} ({ts()})" + (f" — {reason}" if reason else ""), 1)
    flist = "\n".join(f"- {s} {f}" for f, s in sorted(files.items())[:80]) or "- (tidak ada / tanpa git)"
    if len(files) > 80:
        flist += f"\n- … dan {len(files) - 80} file lain"
    text = text.rstrip() + f"\n\n## File berubah\n{flist}\n\n## Ringkasan\n{summary}\n"
    write(sp, text)

    todos = [t for _, tag, t in logs if tag == "TODO"]
    blockers = [t for _, tag, t in logs if tag == "BLOCKER"]
    decisions = [t for _, tag, t in logs if tag == "DECISION"]
    lines = [f"## Session terakhir (otomatis)",
             f"- {sid} · {info.get('start')} → {ts()} · {len(files)} file berubah",
             f"- Ringkasan: {summary}"]
    if decisions:
        lines.append("- Keputusan: " + "; ".join(decisions[-5:]))
    if todos:
        lines.append("- TODO tercatat: " + "; ".join(todos[-8:]))
    if blockers:
        lines.append("- Blocker: " + "; ".join(blockers[-5:]))
    update_memory_auto(p, "\n".join(lines))

    st["last_session"] = sid
    st["last_end_commit"] = head() or st.get("last_end_commit")
    fm = load_filemap(p)
    unmapped = unmapped_new(files, fm)
    compact(p, KEEP_SESSIONS, quiet=True, st=st)
    return {"id": sid, "files": files, "unmapped": unmapped, "auto": is_auto}


def resolve_agent(st: dict, agent):
    if agent:
        return sanitize(agent)
    if len(st["open"]) == 1:
        return next(iter(st["open"]))
    if st["open"]:
        raise SystemExit("Lebih dari satu session terbuka (" + ", ".join(st["open"]) +
                         "). Tambahkan --agent <nama>.")
    return None


def compact(p: Paths, keep: int, quiet=False, st=None) -> int:
    st = st or load_state(p)
    open_ids = {v["id"] for v in st["open"].values()}
    closed = sorted((f for f in p.sessions.glob("*.md") if f.stem not in open_ids),
                    key=lambda f: (f.stat().st_mtime, f.name))
    old = closed[:-keep] if keep > 0 else closed
    if not old:
        return 0
    hist = read(p.history) or "# Riwayat session (ringkas)\n\n"
    p.archive.mkdir(parents=True, exist_ok=True)
    for f in old:
        t = read(f)
        agent = re.search(r"^- agent: (.*)$", t, re.M)
        summ = t.split("## Ringkasan", 1)[1].strip().replace("\n", " ") if "## Ringkasan" in t else "(tanpa ringkasan)"
        hist += f"- {f.stem} ({agent.group(1) if agent else '?'}): {summ[:400]}\n"
        shutil.move(str(f), str(p.archive / f.name))
    write(p.history, hist)
    if not quiet:
        print(f"{len(old)} session lama diringkas ke history.md dan dipindah ke sessions/archive/")
    return len(old)


# ---------------------------------------------------------------------- briefing
PLACEHOLDER_RX = re.compile(r"^\s*-\s*(\(.*\))?\s*$")


def slim_memory(text: str) -> str:
    out = []
    for line in text.splitlines():
        if line.startswith(">") or line.strip().startswith("<!--"):
            continue
        if PLACEHOLDER_RX.match(line):
            continue
        if re.match(r"^- [^:]+: \((?:isi|1–3 kalimat|perintah)[^)]*\)\s*$", line):
            continue
        out.append(line)
    # buang heading yang kosong
    res = []
    for i, line in enumerate(out):
        if line.startswith("## "):
            nxt = next((l for l in out[i + 1:] if l.strip()), "")
            if not nxt or nxt.startswith("## "):
                continue
        res.append(line)
    return re.sub(r"\n{3,}", "\n\n", "\n".join(res)).strip()


def req_summary(p: Paths, show_all=False) -> str:
    text = read(p.req)
    if not text:
        return "(requirements.md belum ada)"
    src = re.search(r"^Sumber: (.*)$", text, re.M)
    items = re.findall(r"^\s*- \[(.)\] (.*)$", text, re.M)
    items = [(s, t) for s, t in items if not t.startswith("R1: <")]
    if not items:
        return (f"Sumber: {src.group(1) if src else '-'}\n"
                "(checklist masih kosong — baca sumber SEKALI lalu isi requirements.md)")
    done = sum(1 for s, _ in items if s.lower() == "x")
    dropped = sum(1 for s, _ in items if s == "-")
    lines = [f"Sumber: {src.group(1) if src else '-'} · selesai {done}/{len(items) - dropped}"]
    for s, t in items:
        if show_all or s not in ("x", "X", "-"):
            lines.append(f"- [{s}] {t}")
    if len(lines) == 1:
        lines.append("(semua requirement selesai)")
    return "\n".join(lines)


def build_brief(p: Paths, st: dict, sid=None, continuing=False, agent=None) -> str:
    py = "python" if os.name == "nt" else "python3"
    cmd = f"{py} .agent-memory/bin/memory.py"
    ag = f" --agent {agent}" if agent else ""
    head_line = f"=== SESSION MEMORY · {ROOT.name} · "
    head_line += (f"melanjutkan session {sid}" if continuing else f"session baru {sid}") if sid else "briefing"
    sec = [head_line + " ===",
           "Sebelum mengerjakan prompt user: cocokkan dengan Requirements & Session terakhir di bawah, "
           "lalu baca HANYA file yang relevan/berubah (jangan membaca seluruh kode).",
           f"Catat: {cmd} log <task|done|decision|todo|blocker|fix|note> \"...\"{ag} · "
           f"req: {cmd} req set R<n> <x|~|-> \"bukti\" · akhiri: {cmd} end{ag} --summary \"...\""]

    mem = slim_memory(read(p.memory))
    sec.append("\n--- MEMORY.md ---\n" + (mem or "(kosong — isi bagian manual MEMORY.md)"))
    sec.append("\n--- Requirements (yang belum selesai) ---\n" + req_summary(p))

    if is_git():
        base = st.get("last_end_commit")
        chg = []
        if base:
            log = git(["log", "--oneline", "-n", "15", f"{base}..HEAD"])
            if log:
                chg.append("Commit sejak session terakhir ditutup:\n" + log)
        status = (git(["status", "--porcelain"]) or "").splitlines()
        status = [s for s in status if MEM not in s]
        if status:
            chg.append(f"Belum di-commit ({len(status)}):\n" + "\n".join(status[:15])
                       + (f"\n… +{len(status) - 15}" if len(status) > 15 else ""))
        if chg:
            sec.append("\n--- Perubahan kode ---\n" + "\n".join(chg))

    fm = load_filemap(p)
    stale = [(f, m) for f, m in fm.items() if file_hash(ROOT / f) != m.get("sha")]
    if stale:
        lines = [f"- {f} — {m.get('purpose', '')}" + (" (HILANG)" if not (ROOT / f).exists() else "")
                 for f, m in stale[:20]]
        sec.append("\n--- File di peta yang berubah sejak terakhir dibaca (baca ulang hanya bila relevan, "
                   f"lalu `{cmd} files refresh <path>`) ---\n" + "\n".join(lines))
    elif fm:
        sec.append(f"\n--- Peta kode: {len(fm)} file terdaftar, tidak ada yang berubah "
                   f"(`{cmd} files list` untuk melihat) ---")
    else:
        sec.append(f"\n--- Peta kode kosong — daftarkan file inti dengan `{cmd} files add` ---")

    others = [f"{a} ({v['id']})" for a, v in st["open"].items() if a != agent]
    if others:
        sec.append("\n--- Session agent lain yang masih terbuka ---\n" + ", ".join(others))

    out = "\n".join(sec)
    if len(out) > BRIEF_LIMIT_CHARS:
        out = out[:BRIEF_LIMIT_CHARS] + ("\n… [briefing dipangkas — MEMORY.md terlalu panjang; "
                                         "ringkas atau pindahkan detail ke history.md]")
    return out + f"\n=== akhir briefing (~{tokens(out)} token) ==="


# ----------------------------------------------------------------------- commands
def need_init(p: Paths, quiet=False) -> bool:
    if p.mem.is_dir():
        return False
    if not quiet:
        print(f"session-memory belum di-init di {ROOT}. Jalankan: memory.py init")
    return True


def read_hook_payload() -> dict:
    try:
        if not sys.stdin.isatty():
            raw = sys.stdin.read()
            return json.loads(raw) if raw.strip() else {}
    except Exception:
        pass
    return {}


def cmd_start(a):
    p = Paths(ROOT)
    if need_init(p, quiet=a.hook):
        return
    st = load_state(p)
    agent = sanitize(a.agent or "agent")
    source = (read_hook_payload().get("source") or "") if a.hook else ""
    continuing = False
    notes = []
    if agent in st["open"]:
        if source in ("resume", "compact") or a.resume:
            continuing = True
        else:
            r = close_session(p, st, agent, reason="session baru dimulai tanpa `end`")
            if r:
                notes.append(f"(session sebelumnya {r['id']} belum ditutup — ditutup otomatis)")
    limit = now() - dt.timedelta(hours=STALE_HOURS)
    for other, info in list(st["open"].items()):
        if other == agent:
            continue
        try:
            started = dt.datetime.strptime(info.get("start", ""), "%Y-%m-%d %H:%M")
        except ValueError:
            started = limit
        if started <= limit:
            close_session(p, st, other, reason=f"terbuka > {STALE_HOURS} jam")
    sid = st["open"][agent]["id"] if continuing else new_session(p, st, agent)
    save_state(p, st)
    print(build_brief(p, st, sid, continuing, agent))
    for n in notes:
        print(n)


def cmd_brief(a):
    p = Paths(ROOT)
    if need_init(p):
        return
    st = load_state(p)
    agent = resolve_agent(st, a.agent)
    sid = st["open"].get(agent, {}).get("id") if agent else None
    print(build_brief(p, st, sid, bool(sid), agent))


def cmd_log(a):
    p = Paths(ROOT)
    if need_init(p):
        return
    st = load_state(p)
    agent = resolve_agent(st, a.agent)
    if not agent or agent not in st["open"]:
        agent = sanitize(a.agent or "agent")
        new_session(p, st, agent)
        print(f"(tidak ada session terbuka — session {st['open'][agent]['id']} dibuat otomatis)")
    text = redact(" ".join(" ".join(a.text).split()))
    if not text:
        print("teks log kosong")
        return
    sp = session_path(p, st["open"][agent]["id"])
    with open(sp, "a", encoding="utf-8", newline="\n") as fh:
        fh.write(f"- {now():%H:%M} [{TAGS[a.type]}] {text}\n")
    st["open"][agent]["last_log"] = ts()
    save_state(p, st)
    if not a.quiet:
        print(f"ok · [{TAGS[a.type]}] dicatat di {sp.relative_to(ROOT).as_posix()}")


def cmd_checkpoint(a):
    p = Paths(ROOT)
    if need_init(p, quiet=a.quiet):
        return
    st = load_state(p)
    agents = [sanitize(a.agent)] if a.agent else list(st["open"])
    for ag in agents:
        info = st["open"].get(ag)
        if not info:
            continue
        info["files"] = changed_files(info.get("start_commit"))
        info["last_checkpoint"] = ts()
        if not a.quiet:
            fm = load_filemap(p)
            unmapped = unmapped_new(info["files"], fm)
            print(f"checkpoint {info['id']}: {len(info['files'])} file berubah sejak awal session")
            if unmapped:
                print("  file baru belum di peta: " + ", ".join(unmapped[:10]))
    save_state(p, st)


def cmd_end(a):
    p = Paths(ROOT)
    if need_init(p, quiet=a.auto):
        return
    st = load_state(p)
    agent = resolve_agent(st, a.agent)
    if not agent or agent not in st["open"]:
        if not a.auto:
            print("Tidak ada session terbuka.")
        return
    summary = " ".join((a.summary or "").split()) or None
    r = close_session(p, st, agent, summary=summary,
                      reason="ditutup hook SessionEnd" if a.auto and not summary else None)
    save_state(p, st)
    if a.auto:
        return
    print(f"Session {r['id']} ditutup · {len(r['files'])} file berubah · "
          f"ringkasan {'OTOMATIS' if r['auto'] else 'tersimpan'}")
    if r["unmapped"]:
        print("File baru yang belum ada di peta (daftarkan yang penting dengan `files add`): "
              + ", ".join(r["unmapped"][:15]))
    print("Pastikan bagian manual MEMORY.md (Status, Keputusan, Langkah berikutnya) sudah mutakhir; "
          "pindahkan TODO yang belum selesai ke 'Langkah berikutnya'.")


def cmd_req(a):
    p = Paths(ROOT)
    if need_init(p):
        return
    text = read(p.req)
    if a.action == "list":
        print(req_summary(p, show_all=a.all))
        return
    if a.action == "add":
        body = redact(" ".join(a.args))
        nums = [int(n) for n in re.findall(r"^\s*- \[.\] R(\d+):", text, re.M)]
        rid = f"R{max(nums, default=0) + 1}"
        write(p.req, text.rstrip() + f"\n- [ ] {rid}: {body}\n")
        print(f"{rid} ditambahkan")
        return
    if a.action == "set":
        if len(a.args) < 2:
            print("pakai: req set R<n> <x|~|-|todo> [catatan]")
            return
        rid, status, note = a.args[0].upper(), a.args[1].lower(), redact(" ".join(a.args[2:]))
        if status not in REQ_STATUS:
            print(f"status tidak dikenal: {status}. Pilih: x, ~, -, todo")
            return
        rx = re.compile(r"^(\s*- \[).(\] " + re.escape(rid) + r":\s*)(.*)$", re.M)
        m = rx.search(text)
        if not m:
            print(f"{rid} tidak ditemukan di requirements.md")
            return
        body = m.group(3)
        if note:
            body = body.split(" — ", 1)[0] + " — " + note
        text = text[:m.start()] + m.group(1) + REQ_STATUS[status] + m.group(2) + body + text[m.end():]
        write(p.req, text)
        print(f"{rid} → [{REQ_STATUS[status]}]")


def cmd_files(a):
    p = Paths(ROOT)
    if need_init(p):
        return
    fm = load_filemap(p)
    if a.action == "add":
        if not a.args:
            print('pakai: files add <path> "fungsi singkat"')
            return
        path, purpose = rel(a.args[0]), redact(" ".join(a.args[1:]))
        if not (ROOT / path).exists():
            print(f"peringatan: {path} tidak ada (tetap dicatat)")
        fm[path] = {"purpose": purpose or fm.get(path, {}).get("purpose", ""), "sha": file_hash(ROOT / path)}
        save_filemap(p, fm)
        print(f"peta: {path}")
    elif a.action == "rm":
        for f in a.args:
            fm.pop(rel(f), None)
        save_filemap(p, fm)
        print("dihapus dari peta")
    elif a.action == "refresh":
        targets = [rel(f) for f in a.args] or list(fm)
        for f in targets:
            if f in fm:
                fm[f]["sha"] = file_hash(ROOT / f)
        save_filemap(p, fm)
        print(f"{len(targets)} entri peta diperbarui")
    elif a.action == "stale":
        stale = [f for f, m in fm.items() if file_hash(ROOT / f) != m.get("sha")]
        print("\n".join(stale) or "(tidak ada)")
    else:
        if not fm:
            print("(peta kosong)")
        for f, m in fm.items():
            flag = "" if file_hash(ROOT / f) == m.get("sha") else "  *berubah*"
            print(f"- {f} — {m.get('purpose', '')}{flag}")


def cmd_compact(a):
    p = Paths(ROOT)
    if need_init(p):
        return
    if not compact(p, a.keep):
        print("Tidak ada yang perlu diringkas.")


def cmd_status(a):
    p = Paths(ROOT)
    if need_init(p):
        return
    st = load_state(p)
    brief = build_brief(p, st)
    n_sess = len(list(p.sessions.glob("*.md")))
    n_arch = len(list(p.archive.glob("*.md")))
    opened = ", ".join("%s=%s" % (k, v["id"]) for k, v in st["open"].items()) or "-"
    print(f"root: {ROOT}\nversi: {VERSION}\nsession: {n_sess} aktif/terbaru, {n_arch} arsip"
          f"\nsession terbuka: {opened}"
          f"\nsession terakhir ditutup: {st.get('last_session') or '-'}"
          f"\nukuran briefing: ~{tokens(brief)} token · MEMORY.md {len(read(p.memory).splitlines())} baris"
          f"\npeta kode: {len(load_filemap(p))} file")


# --------------------------------------------------------------------------- main
def main(argv=None):
    global ROOT
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass
    ap = argparse.ArgumentParser(prog="memory.py", description="session-memory: memori lintas-session coding agent")
    ap.add_argument("--root", help="root proyek (default: auto-deteksi)")
    ap.add_argument("--version", action="version", version=VERSION)
    sub = ap.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("init", help="siapkan .agent-memory/ + bootstrap AGENTS.md/CLAUDE.md + hook")
    s.add_argument("--req", help="path dokumen requirements awal")
    s.add_argument("--agents", default="auto",
                   help="auto|all|daftar koma: agents,claude,gemini,cursor,copilot,windsurf,cline")
    s.add_argument("--python", help="perintah python untuk bootstrap/hook (default: python di Windows, python3 lainnya)")
    s.add_argument("--no-hooks", action="store_true", help="jangan pasang hook Claude Code")
    s.set_defaults(fn=cmd_init)

    s = sub.add_parser("start", help="buka session + cetak briefing")
    s.add_argument("--agent", default=os.environ.get("SESSION_MEMORY_AGENT"))
    s.add_argument("--hook", action="store_true", help="dipanggil dari hook (baca payload stdin)")
    s.add_argument("--resume", action="store_true", help="lanjutkan session terbuka milik agent ini")
    s.set_defaults(fn=cmd_start)

    s = sub.add_parser("brief", help="cetak briefing tanpa membuka session")
    s.add_argument("--agent")
    s.set_defaults(fn=cmd_brief)

    s = sub.add_parser("log", help="catat progres ke session berjalan")
    s.add_argument("type", choices=list(TAGS))
    s.add_argument("text", nargs="+")
    s.add_argument("--agent", default=os.environ.get("SESSION_MEMORY_AGENT"))
    s.add_argument("--quiet", action="store_true")
    s.set_defaults(fn=cmd_log)

    s = sub.add_parser("checkpoint", help="rekam file yang berubah (dipakai hook)")
    s.add_argument("--agent")
    s.add_argument("--quiet", action="store_true")
    s.set_defaults(fn=cmd_checkpoint)

    s = sub.add_parser("end", help="tutup session + perbarui MEMORY.md")
    s.add_argument("--agent", default=os.environ.get("SESSION_MEMORY_AGENT"))
    s.add_argument("--summary", "-s")
    s.add_argument("--auto", action="store_true", help="mode hook: diam, ringkasan otomatis bila belum ada")
    s.set_defaults(fn=cmd_end)

    s = sub.add_parser("req", help="checklist requirements: list|add|set")
    s.add_argument("action", choices=["list", "add", "set"])
    s.add_argument("args", nargs="*")
    s.add_argument("--all", action="store_true")
    s.set_defaults(fn=cmd_req)

    s = sub.add_parser("files", help="peta kode: list|add|rm|refresh|stale")
    s.add_argument("action", choices=["list", "add", "rm", "refresh", "stale"], nargs="?", default="list")
    s.add_argument("args", nargs="*")
    s.set_defaults(fn=cmd_files)

    s = sub.add_parser("compact", help="ringkas session lama ke history.md")
    s.add_argument("--keep", type=int, default=KEEP_SESSIONS)
    s.set_defaults(fn=cmd_compact)

    s = sub.add_parser("status", help="ringkasan kondisi memori")
    s.set_defaults(fn=cmd_status)

    a = ap.parse_args(argv)
    ROOT = Path(a.root).resolve() if a.root else find_root()
    silent = getattr(a, "hook", False) or getattr(a, "quiet", False) or getattr(a, "auto", False)
    try:
        a.fn(a)
    except Exception as e:  # hook tidak boleh menggagalkan session agent
        if silent:
            print(f"[session-memory] dilewati: {e}", file=sys.stderr)
            return 0
        raise
    return 0


if __name__ == "__main__":
    sys.exit(main())
