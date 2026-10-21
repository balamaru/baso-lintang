"""Create local-only configuration without overwriting existing credentials."""

from pathlib import Path
import secrets

root = Path(__file__).resolve().parents[1]
target = root / ".env"
values = {
    "POSTGRES_PASSWORD": secrets.token_hex(24),
    "SESSION_SECRET": secrets.token_hex(32),
    "HASH_SALT": secrets.token_hex(32),
    "ADMIN_PASSWORD": secrets.token_hex(24),
    # Public Cloudflare testing credentials, never use for production.
    "TURNSTILE_SITE_KEY": "1x00000000000000000000AA",
    "TURNSTILE_SECRET": "1x0000000000000000000000000000000AA",
    "BASE_URL": "https://localhost:8443",
}
lines = []
for line in (root / ".env.example").read_text(encoding="utf-8").splitlines():
    key = line.partition("=")[0]
    if key == "DATABASE_URL":
        line = "# DATABASE_URL diatur Compose; PostgreSQL tidak diekspos ke host."
    elif key in values:
        line = key + "=" + values[key]
    lines.append(line)
try:
    with target.open("x", encoding="utf-8", newline="\n") as output:
        output.write("\n".join(lines) + "\n")
    target.chmod(0o600)
except FileExistsError:
    raise SystemExit(".env sudah ada; tidak ditimpa. Sesuaikan konfigurasi lokal secara manual.")
print(".env lokal dibuat dengan rahasia acak dan kunci uji CAPTCHA. Rahasia tidak dicetak.")
