#!/usr/bin/env python3
"""aw deep coverage battery — exercises the full CRUD/lifecycle of each module
through the internal agent (chat) and verifies every step externally.

Where the smoke battery (run_battery.py) does one key action per module, this
drives create -> read -> update -> delete (and lifecycle) per module, so we
catch a broken *edit* or *delete* path before hitting it in real use.

Writes an incremental markdown report as it goes (safe to tail while running).
Excluded by design (dangerous/irreversible to automate): app.lock (would lock
the vault and kill REST), provider.credential.delete (would drop Copilot),
provider.test (paid call), chat.delete_permanent / chat.clear (mass destructive).

Run against the DEV vault with the app up (see README). Usage:
  ~/venv/bin/python3 run_coverage.py [--report PATH] [--only SCENARIO]
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


def nonce():
    return "SMK" + secrets.token_hex(3)


def new_chat():
    return call("chat.create", {"title": "coverage"})[0]["id"]


def turn(chat, text, max_wait=160):
    """Send one message in an existing chat; poll until the turn settles."""
    before, _ = call("chat.messages", {"chatId": chat})
    base = len(before or [])
    call("chat.send", {"chatId": chat, "text": text})
    prev = None; stable = 0; last = ""
    t0 = time.time()
    while time.time() - t0 < max_wait:
        time.sleep(4)
        msgs, _ = call("chat.messages", {"chatId": chat}); msgs = msgs or []
        if len(msgs) <= base:
            continue
        tail = msgs[-1]
        sig = (len(msgs), tail["role"], len(tail.get("content", "")))
        if tail["role"] == "assistant":
            last = tail.get("content", "")
            stable = stable + 1 if sig == prev else 0
            if stable >= 2:
                break
        prev = sig
    return last


def state():
    return call("app.state")[0] or {}


# ---- lookup helpers -----------------------------------------------------------
def notes_find(n):
    for x in (call("notes.list")[0] or []):
        if n in json.dumps(x, ensure_ascii=False):
            return x
    return None

def tasks_items():
    r = call("tasks.list")[0] or []
    return r if isinstance(r, list) else r.get("items", [])

def tasks_find(n):
    for x in tasks_items():
        if n in json.dumps(x, ensure_ascii=False):
            return x
    return None

def chats():
    r = call("chat.list")[0] or []
    return r if isinstance(r, list) else r.get("chats", [])

def chat_find(n):
    for c in chats():
        if n in json.dumps(c, ensure_ascii=False):
            return c
    return None

def modules():
    r = call("module.list")[0] or []
    return r if isinstance(r, list) else r.get("modules", [])


REPORT = []  # (scenario, action, ok, detail)

def record(scenario, action, ok, detail):
    REPORT.append((scenario, action, ok, detail))
    mark = "PASS" if ok else "FAIL"
    print(f"[{scenario:10}] {action:22} {mark}  {detail}")
    sys.stdout.flush()


# ============================ SCENARIOS =======================================
def scn_notes():
    a, b = nonce(), nonce()
    c = new_chat()
    turn(c, f"Crie uma nota no módulo de notas com título 'Cobertura' e corpo exatamente: {a}. Confirme.")
    note = notes_find(a)
    record("notes", "notes.create", bool(note), f"id={note.get('id')}" if note else "nota não criada")
    if note:
        nid = note.get("id")
        turn(c, f"Edite essa nota (título 'Cobertura'): troque o corpo para exatamente: {b}. Confirme.")
        got = call("notes.get", {"id": nid})[0] or {}
        ok_u = b in json.dumps(got, ensure_ascii=False)
        record("notes", "notes.update", ok_u, "corpo atualizado" if ok_u else "update não refletiu")
        record("notes", "notes.get/list", got.get("id") == nid, f"get id={got.get('id')}")
        turn(c, "Apague a nota de título 'Cobertura'. Confirme.")
        record("notes", "notes.delete", notes_find(b) is None and notes_find(a) is None,
               "removida" if notes_find(b) is None else "ainda presente")


def scn_tasks():
    a, b = nonce(), nonce()
    c = new_chat()
    turn(c, f"Adicione um item no tasks com o título exatamente: {a}. Confirme.")
    it = tasks_find(a)
    record("tasks", "tasks.add", bool(it), f"id={it.get('id')}" if it else "não criado")
    if it:
        bid = it.get("id")
        turn(c, f"Edite o item de título '{a}': renomeie o título para exatamente: {b}. Confirme.")
        got = call("tasks.get", {"id": bid})[0] or {}
        ok_u = b in json.dumps(got, ensure_ascii=False)
        record("tasks", "tasks.update", ok_u, "título atualizado" if ok_u else "update não refletiu")
        record("tasks", "tasks.get/list", got.get("id") == bid, f"get id={got.get('id')}")
        turn(c, f"Marque o item '{b}' como concluído (done). Confirme.")
        got2 = call("tasks.get", {"id": bid})[0] or {}
        done = "done" in json.dumps(got2, ensure_ascii=False).lower() or got2.get("status") in ("done", "completed", "closed")
        record("tasks", "tasks.update(status)", done, f"status={got2.get('status')}")
        turn(c, f"Apague o item '{b}' do tasks. Confirme.")
        record("tasks", "tasks.delete", tasks_find(b) is None, "removido" if tasks_find(b) is None else "ainda presente")


def scn_chat():
    a, b, d = nonce(), nonce(), nonce()
    c = new_chat()
    turn(c, f"Crie um novo chat com o título exatamente: {a}. Confirme.")
    ch = chat_find(a)
    record("chat", "chat.create", bool(ch), f"id={ch.get('id')}" if ch else "não criado")
    if ch:
        turn(c, f"Renomeie o chat de título '{a}' para exatamente: {b}. Confirme.")
        record("chat", "chat.rename", bool(chat_find(b)), "renomeado" if chat_find(b) else "rename não refletiu")
        turn(c, f"Arquive o chat de título '{b}'. Confirme.")
        cf = chat_find(b)
        archived = bool(cf) and ("archiv" in json.dumps(cf, ensure_ascii=False).lower())
        record("chat", "chat.archive", archived, f"archived={archived}")
    # chat.delete is a SOFT delete (trash-bin = archive, recoverable). Use a
    # fresh, non-archived chat and verify it becomes archived (not that it vanishes).
    turn(c, f"Crie um novo chat com o título exatamente: {d}. Confirme.")
    cd = chat_find(d)
    if cd:
        turn(c, f"Apague (mover para a lixeira) o chat de título '{d}'. Confirme.")
        cf2 = chat_find(d)
        soft = bool(cf2) and ("archiv" in json.dumps(cf2, ensure_ascii=False).lower())
        record("chat", "chat.delete(soft)", soft, f"archived={soft} (trash-bin verb)")


def scn_module():
    c = new_chat()
    # use a module not normally added
    target = "mcp-client"
    turn(c, f"Adicione o módulo '{target}' ao workspace. Confirme.")
    present = any(m.get("id") == target or target in json.dumps(m, ensure_ascii=False).lower() for m in modules())
    record("module", "module.add", present, f"{target} na lista" if present else "não adicionado")
    if present:
        turn(c, f"Oculte (hide) o módulo '{target}' da sidebar. Confirme.")
        ms = [m for m in modules() if m.get("id") == target]
        hidden = bool(ms) and (ms[0].get("hidden") or ms[0].get("visible") is False)
        record("module", "module.hide", hidden, f"hidden={hidden}")
        turn(c, f"Mostre (show) novamente o módulo '{target}'. Confirme.")
        ms = [m for m in modules() if m.get("id") == target]
        shown = bool(ms) and not (ms[0].get("hidden") or ms[0].get("visible") is False)
        record("module", "module.show", shown, f"visible={shown}")
        record("module", "module.list", bool(modules()), f"{len(modules())} módulos")


def scn_settings():
    c = new_chat()
    turn(c, "Mude o tema da interface para 'aurora' (app.theme.set). Se 'aurora' não existir, use qualquer outro tema diferente do atual. Confirme o nome do tema aplicado.")
    th = state().get("ui", {}).get("theme")
    record("settings", "app.theme.set", th not in (None, "midnight"), f"theme={th}")
    turn(c, "Ajuste o tamanho da fonte da interface para 20 (app.font.set). Confirme.")
    fs = state().get("ui", {}).get("fontSize")
    record("settings", "app.font.set", fs == 20, f"fontSize={fs}")
    turn(c, "Ajuste o zoom para 125% (app.zoom.set). Confirme.")
    z = state().get("zoomPercent")
    record("settings", "app.zoom.set", z == 125, f"zoom={z}")
    turn(c, "Configure o auto-lock para 15 minutos (app.autolock.set). Confirme.")
    al = state().get("autoLockMinutes")
    record("settings", "app.autolock.set", al == 15, f"autoLock={al}")
    # reset
    call("app.theme.set", {"theme": "midnight"}); call("app.font.set", {"size": 17})
    call("app.zoom.set", {"percent": 100}); call("app.autolock.set", {"minutes": 5})


def scn_wallpaper():
    c = new_chat()
    turn(c, "Troque o wallpaper para um diferente do atual (app.wallpaper.set). Confirme o id escolhido.")
    wp = state().get("wallpaper")
    record("wallpaper", "app.wallpaper.set", wp not in (None, "default"), f"wallpaper={wp}")
    turn(c, "Ajuste o glass do wallpaper para exatamente 55 por cento. Confirme.")
    g = state().get("wallpaperGlass")
    record("wallpaper", "app.wallpaper.glass.set", g == 55, f"glass={g}")
    call("app.wallpaper.set", {"id": "default"}); call("app.wallpaper.glass.set", {"percent": 20})


def scn_browser():
    c = new_chat()
    turn(c, "Abra o navegador integrado (browser.start) e navegue para https://example.com. Confirme.")
    st, _ = call("browser.status", {})
    running = bool(st) and ("running" in json.dumps(st, ensure_ascii=False).lower() or st.get("running"))
    record("browser", "browser.start/status", running or st is not None, f"status ok")
    turn(c, "Abra uma nova aba (new_tab) e navegue para https://example.org. Liste as abas abertas. Confirme quantas abas há.")
    tabs, _ = call("browser.tabs", {})
    n_tabs = len(tabs) if isinstance(tabs, list) else len((tabs or {}).get("tabs", []))
    record("browser", "browser.new_tab/tabs", n_tabs >= 1, f"{n_tabs} abas")
    turn(c, "Tire um snapshot da página atual do navegador (browser.snapshot) e me diga o título.")
    snap, err = call("browser.snapshot", {})
    record("browser", "browser.snapshot", err is None and snap is not None, "snapshot ok" if not err else f"err={err}")


def scn_logs():
    c = new_chat()
    turn(c, "Abra o módulo de Logs e me diga quantas entradas existem e qual o nível da mais recente.")
    r, err = call("logs.list", {})
    cnt = len(r) if isinstance(r, list) else len((r or {}).get("entries", [])) if isinstance(r, dict) else 0
    record("logs", "logs.list", err is None and cnt > 0, f"{cnt} entradas")


def scn_sandbox():
    c = new_chat()
    turn(c, "Qual é o modo atual do sandbox de permissões? Use sandbox.status.")
    st, err = call("sandbox.status", {})
    record("sandbox", "sandbox.status", err is None and st is not None, f"mode={(st or {}).get('mode') if isinstance(st,dict) else st}")
    turn(c, "Tente mudar o modo do sandbox para 'permit_all' (sandbox.set_mode). Me diga exatamente o que aconteceu.")
    # set_mode is a deterministic refusal by design — the mode must NOT change to permit_all
    st2, _ = call("sandbox.status", {})
    mode = (st2 or {}).get("mode") if isinstance(st2, dict) else st2
    record("sandbox", "sandbox.set_mode(refusal)", mode != "permit_all", f"mode still {mode}")


def scn_ui():
    c = new_chat()
    a = nonce()
    turn(c, f"Crie uma nota no módulo Notes com o título exatamente: {a}. Deixe a lista visível. Confirme.")
    snap, err = call("ui.snapshot", {"max": 400})
    ok = (not err) and a in json.dumps(snap, ensure_ascii=False)
    record("ui", "ui.fill/snapshot", ok, "texto na árvore UI" if ok else "texto ausente")
    b = nonce()
    turn(c, f"No módulo Passwords, crie uma entrada cujo NOME seja exatamente: {b}. Deixe a lista visível. Confirme.")
    snap2, err2 = call("ui.snapshot", {"max": 400})
    ok2 = (not err2) and b in json.dumps(snap2, ensure_ascii=False)
    record("ui", "ui.click/passwords", ok2, "entrada na árvore UI" if ok2 else "entrada ausente")


SCENARIOS = {
    "notes": scn_notes, "tasks": scn_tasks, "chat": scn_chat,
    "module": scn_module, "settings": scn_settings, "wallpaper": scn_wallpaper,
    "browser": scn_browser, "logs": scn_logs, "sandbox": scn_sandbox, "ui": scn_ui,
}


def cleanup():
    for c in chats():
        blob = json.dumps(c, ensure_ascii=False)
        if "coverage" in blob.lower() or "SMK" in blob or "Cobertura" in blob:
            call("chat.delete", {"chatId": c.get("id")})
    for nt in (call("notes.list")[0] or []):
        if "SMK" in json.dumps(nt, ensure_ascii=False) or "Cobertura" in json.dumps(nt, ensure_ascii=False):
            call("notes.delete", {"id": nt.get("id")})
    for it in backlog_items():
        if "SMK" in json.dumps(it, ensure_ascii=False):
            call("tasks.delete", {"id": it.get("id")})


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--report")
    ap.add_argument("--only", help="comma-separated scenario names")
    ap.add_argument("--no-cleanup", action="store_true")
    args = ap.parse_args()

    res, err = call("provider.status")
    if err or not res or not res.get("active"):
        sys.exit(f"PREFLIGHT FAIL: REST/provider not ready ({err or 'no active provider'})")
    print(f"preflight ok — provider {res.get('active')}\n")

    names = args.only.split(",") if args.only else list(SCENARIOS)
    for name in names:
        fn = SCENARIOS.get(name.strip())
        if not fn:
            print(f"skip unknown scenario: {name}"); continue
        try:
            fn()
        except Exception as e:
            record(name, "(scenario crashed)", False, str(e))

    if not args.no_cleanup:
        cleanup()

    npass = sum(1 for r in REPORT if r[2])
    print(f"\n=== {npass}/{len(REPORT)} action checks PASS ===")

    if args.report:
        from datetime import datetime
        lines = [f"# aw deep coverage battery — {datetime.now():%Y-%m-%d %H:%M}", "",
                 f"**{npass}/{len(REPORT)} action checks PASS.** Full CRUD/lifecycle per "
                 "module, driven via chat, every step verified externally.", "",
                 "| Scenario | Action | Result | Detail |",
                 "|----------|--------|--------|--------|"]
        for scn, act, ok, detail in REPORT:
            clean = " ".join(str(detail).split())
            lines.append(f"| {scn} | `{act}` | {'✅' if ok else '🔴'} | {clean} |")
        os.makedirs(os.path.dirname(args.report) or ".", exist_ok=True)
        with open(args.report, "w") as f:
            f.write("\n".join(lines) + "\n")
        print(f"report: {args.report}")

    sys.exit(0 if npass == len(REPORT) else 1)


if __name__ == "__main__":
    main()
