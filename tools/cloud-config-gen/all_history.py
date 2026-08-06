#!/usr/bin/env python3
# 列出所有调度任务最近历史（task, status, started），确认各引擎健康。
# 服务器端：python3 /root/unimap-config/all_history.py
import json
import subprocess
import urllib.request

LIST = "http://127.0.0.1:8448/api/v1/scheduler/tasks"
HIST = "http://127.0.0.1:8448/api/v1/scheduler/history"


def token():
    return subprocess.check_output(
        ["docker", "exec", "unimap-unimap-1", "printenv", "UNIMAP_ADMIN_TOKEN"],
        text=True,
    ).strip()


TOKEN = token()


def api(path):
    req = urllib.request.Request(
        path,
        headers={"X-Admin-Token": TOKEN, "Origin": "http://127.0.0.1:8448"},
    )
    try:
        with urllib.request.urlopen(req) as r:
            return json.loads(r.read().decode() or "[]")
    except Exception as e:
        return {"_err": str(e)}


def main():
    data = api(LIST)
    tasks = data if isinstance(data, list) else data.get("tasks", data.get("data", []))
    for t in sorted(tasks, key=lambda x: x.get("name", "")):
        name = t.get("name", "")
        engs = ",".join((t.get("payload") or {}).get("engines") or [])
        cron = t.get("cron_expr")
        h = api(HIST + "?task_id=%s&limit=2" % t.get("id"))
        if not isinstance(h, list) or not h:
            print("%-26s [%-18s] cron=%-14s | no history" % (name, engs, cron))
            continue
        rec = h[0]
        st = rec.get("status")
        started = (rec.get("started_at") or rec.get("started") or "")[:19]
        err = (rec.get("error") or "")[:40]
        print("%-26s [%-18s] cron=%-14s | %-8s %s %s"
              % (name, engs, cron, st, started, err))


if __name__ == "__main__":
    main()
