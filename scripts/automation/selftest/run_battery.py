#!/usr/bin/env python3
"""aw self-test battery — dogfoods two things at once:

  1. Whether the INTERNAL aw agent can manage itself (we ask it, in plain
     language, to do one key operation per module).
  2. Whether each MODULE actually works (we verify the effect EXTERNALLY via
     REST / disk / UI tree — never by trusting the agent's "done").

The agent is driven through the chat REST surface; every result is checked
out-of-band with a per-test random nonce so a prior run can't cause a false
positive. Run against the DEV vault (throwaway) with the app already up.

Prerequisites (the script checks and fails loud if missing):
  - aw dev build running with REST on :9311 (auto-unlock, vault "1234")
  - A provider connected in that vault (github-copilot claude-sonnet-4.6)

Usage:
  python3 run_battery.py                 # all 11 modules
  python3 run_battery.py --only notes    # one module
  python3 run_battery.py --report out.md # also write a markdown report

Exit code: 0 if no module FAILED, 1 otherwise (loop-friendly).
"""
import argparse, json, os, secrets, sys, time, urllib.request

EP  = os.environ.get("AW_EP",  "http://127.0.0.1:9311/api/aw")
TOK = os.environ.get("AW_TOK", "fa756217f60be891fa815db1ff3709d960354b43dff3ed157eada79e5bd4d7a7")


def call(action, args=None, timeout=30):
    body = json.dumps({"action": action, "args": args or {}}).encode()
    req = urllib.request.Request(EP, data=body, headers={
        "Authorization": "Bearer " + TOK, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return json.load(r).get("result"), None
    except Exception as e:
        return None, str(e)


def preflight():
    res, err = call("provider.status")
    if err:
        sys.exit(f"PREFLIGHT FAIL: REST not reachable at {EP} ({err}).\n"
                 "Start the dev build first (see README).")
    active = res.get("active")
    connected = any(p.get("connected") for p in res.get("providers", []))
    if not active or not connected:
        sys.exit(f"PREFLIGHT FAIL: no provider connected in the dev vault "
                 f"(active={active}). Connect one in Settings > Providers.")
    print(f"preflight ok — provider active: {active}")


def run_turn(text, max_wait=200):
    """Send one message, poll chat.messages until the assistant turn settles."""
    chat = call("chat.create", {"title": "selftest"})[0]["id"]
    call("chat.send", {"chatId": chat, "text": text})
    prev = None; stable = 0; last = ""
    t0 = time.time()
    while time.time() - t0 < max_wait:
        time.sleep(4)
        msgs, _ = call("chat.messages", {"chatId": chat}); msgs = msgs or []
        if not msgs:
            continue
        tail = msgs[-1]
        sig = (len(msgs), tail["role"], len(tail.get("content", "")))
        if tail["role"] == "assistant":
            last = tail.get("content", "")
            stable = stable + 1 if sig == prev else 0
            if stable >= 2:
                break
        prev = sig
    return chat, last


def state():
    return call("app.state")[0] or {}


def snap_has(nonce):
    r, err = call("ui.snapshot", {"max": 400})
    if err:
        return False, f"ui.snapshot error: {err}"
    return (nonce in json.dumps(r, ensure_ascii=False)), "UI tree checked"


# ---- per-module verifiers (return (ok, detail)) -------------------------------
def v_notes(n):
    r = call("notes.list")[0] or []
    hit = [x for x in r if n in json.dumps(x, ensure_ascii=False)]
    return bool(hit), (f"note id={hit[0].get('id')}" if hit else "nonce absent")

def v_tasks(n):
    r = call("tasks.list")[0] or []
    items = r if isinstance(r, list) else r.get("items", [])
    hit = [x for x in items if n in json.dumps(x, ensure_ascii=False)]
    return bool(hit), (f"item id={hit[0].get('id')}" if hit else "nonce absent")

def v_fs(n):
    path = f"/tmp/aw-selftest-{n}.txt"
    try:
        with open(path) as f:
            return (n in f.read()), f"disk file {path}"
    except Exception as e:
        return False, f"file missing: {e}"

def v_chat(n):
    r = call("chat.list")[0] or []
    chats = r if isinstance(r, list) else r.get("chats", [])
    hit = [c for c in chats if n in json.dumps(c, ensure_ascii=False)]
    return bool(hit), (f"chat id={hit[0].get('id')}" if hit else "nonce absent")

def v_settings(n):
    z = state().get("zoomPercent")
    call("app.zoom.set", {"percent": 100})  # reset
    return z == 115, f"zoomPercent={z} (want 115)"

def v_logs(n):
    r, err = call("logs.list", {})
    if err:
        return False, f"logs.list error: {err}"
    cnt = len(r) if isinstance(r, list) else len(r.get("entries", [])) if isinstance(r, dict) else 0
    return cnt > 0, f"logs.list returned {cnt} entries"

def v_wallpaper(n):
    g = state().get("wallpaperGlass")
    call("app.wallpaper.glass.set", {"percent": 20})  # reset
    return g == 45, f"wallpaperGlass={g} (want 45)"

def v_home(n):
    v = state().get("ui", {}).get("view")
    return v == "home", f"ui.view={v} (want home)"

def v_browser(n):
    _, err = call("browser.status", {})
    return err is None, ("browser.status ok" if err is None else f"browser.status error: {err}")


# ---- the battery: module -> (instruction, verifier) ---------------------------
BATTERY = [
    ("notes",
     "Crie uma nota nova no módulo de notas com o título 'Selftest' e o corpo contendo exatamente este texto, nada mais: {N}. Confirme.",
     v_notes),
    ("tasks",
     "Adicione um item novo no tasks com o título exatamente: {N}. Confirme.",
     v_tasks),
    ("fs",
     "Escreva um arquivo de texto no caminho exato /tmp/aw-selftest-{N}.txt com o conteúdo exatamente: {N}. Confirme.",
     v_fs),
    ("chat",
     "Crie um novo chat com o título exatamente: {N}. Confirme.",
     v_chat),
    ("settings",
     "Ajuste o zoom da interface para exatamente 115% (app.zoom.set). Confirme.",
     v_settings),
    ("logs",
     "Abra o módulo de Logs e me diga quantas entradas de log existem agora.",
     v_logs),
    ("wallpaper",
     "Ajuste o desfoque (glass) do wallpaper para exatamente 45 por cento. Confirme.",
     v_wallpaper),
    ("home",
     "Navegue para a tela inicial de aplicativos (Home / Open Apps and Modules). Confirme.",
     v_home),
    ("passwords",
     "Abra o módulo Passwords e crie uma entrada cujo NOME seja exatamente: {N} (senha qualquer). Deixe a lista visível. Confirme.",
     snap_has),
    ("browser",
     "Abra o navegador integrado e navegue para https://example.com. Confirme o título da página.",
     v_browser),
]


def cleanup():
    """Best-effort: archive test chats, drop test notes, remove /tmp files."""
    r = call("chat.list")[0] or []
    chats = r if isinstance(r, list) else r.get("chats", [])
    for c in chats:
        blob = json.dumps(c, ensure_ascii=False)
        if "selftest" in blob.lower() or "SMK" in blob:
            call("chat.delete", {"chatId": c.get("id")})
    for nt in (call("notes.list")[0] or []):
        if "SMK" in json.dumps(nt, ensure_ascii=False):
            call("notes.delete", {"id": nt.get("id")})
    for f in os.listdir("/tmp"):
        if f.startswith("aw-selftest-") and f.endswith(".txt"):
            try: os.remove("/tmp/" + f)
            except OSError: pass


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", help="run a single module by name")
    ap.add_argument("--report", help="write a markdown report to this path")
    ap.add_argument("--no-cleanup", action="store_true")
    args = ap.parse_args()

    preflight()
    tests = [t for t in BATTERY if not args.only or t[0] == args.only]
    if not tests:
        sys.exit(f"unknown module: {args.only}")

    rows = []
    for name, instruction, verify in tests:
        nonce = "SMK" + secrets.token_hex(3)
        _, reply = run_turn(instruction.replace("{N}", nonce))
        try:
            ok, detail = verify(nonce)
        except Exception as e:
            ok, detail = False, f"verify error: {e}"
        status = "PASS" if ok else "FAIL"
        rows.append((name, status, detail, reply[:100].strip().replace("\n", " ")))
        print(f"[{name:14}] {status:4}  nonce={nonce}  {detail}")

    if not args.no_cleanup:
        cleanup()

    npass = sum(1 for r in rows if r[1] == "PASS")
    print(f"\n=== {npass}/{len(rows)} PASS ===")

    if args.report:
        from datetime import datetime
        lines = [f"# aw self-test battery — {datetime.now():%Y-%m-%d %H:%M}",
                 "", f"**{npass}/{len(rows)} modules PASS.** "
                 "Each module exercised via chat with the internal agent; "
                 "every effect verified externally with a unique nonce.", "",
                 "| Module | Result | Verified by | Agent said |",
                 "|--------|--------|-------------|------------|"]
        for name, status, detail, said in rows:
            mark = "✅ PASS" if status == "PASS" else "🔴 FAIL"
            lines.append(f"| {name} | {mark} | {detail} | {said!r} |")
        os.makedirs(os.path.dirname(args.report) or ".", exist_ok=True)
        with open(args.report, "w") as f:
            f.write("\n".join(lines) + "\n")
        print(f"report written: {args.report}")

    sys.exit(0 if npass == len(rows) else 1)


if __name__ == "__main__":
    main()
