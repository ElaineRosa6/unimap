#!/usr/bin/env python3
# 精简 hunter 任务：6 任务(b1-b5+a) -> 4 任务(b1/b2/b3+a)，每段覆盖高价值词。
# 低价值城市词由 fofa/daydaymap 全词覆盖。
# 每日 500 条配额（每次查询 ≤100 条→每天 ≤5 次）：hunter 每天 1 次，4 次查询(400 条)，
# 错峰 9:00/9:10/9:20/9:40，15:00 不跑 hunter（fofa/quake/daydaymap 仍每天两次）。
# 真实目标查询在 ynmobile_targets.json（gitignored，不入库）。服务器端先同步该文件：
#   scp tools/cloud-config-gen/ynmobile_targets.json root@SERVER:/root/unimap-config/
#   python3 /root/unimap-config/reduce_hunter.py
import json
import os
import subprocess
import urllib.request

API = "http://127.0.0.1:8448/api/v1/scheduler/tasks"
HERE = os.path.dirname(os.path.abspath(__file__))


def load_targets(name):
    with open(os.path.join(HERE, "ynmobile_targets.json"), encoding="utf-8") as f:
        for t in json.load(f):
            if t["name"] == name:
                return t["query"]
    raise KeyError("no target query for %s" % name)


Q_A = load_targets("hunter_ynmobile_a")
Q_B1 = load_targets("hunter_ynmobile_b")
Q_B2 = load_targets("hunter_ynmobile_b2")
Q_B3 = load_targets("hunter_ynmobile_b3")

# (name, id, query, cron) —— cron 每天 1 次（9 点段）
UPDATE = [
    ("hunter_ynmobile_b",  "1ddedd7e-3b79-41bb-8452-cc7984aa833b", Q_B1, "0 9 * * *"),
    ("hunter_ynmobile_b2", "495b7cc9-8cda-45ca-9e2a-a7b6bfbc964e", Q_B2, "10 9 * * *"),
    ("hunter_ynmobile_b3", "c92046e8-7143-4b68-abf6-3505697c48b4", Q_B3, "20 9 * * *"),
    ("hunter_ynmobile_a",  "e06cff3a-e9bf-408d-98df-633b319a8794", Q_A, "40 9 * * *"),
]

DELETE = [
    ("hunter_ynmobile_b4", "5c0ff896-fc90-4ea4-8baa-78497eeb6e7a"),
    ("hunter_ynmobile_b5", "ea934dbf-189a-4fe9-bef6-9ff627db803b"),
]

ALLOWED = [
    "id", "name", "type", "enabled", "cron_expr", "payload",
    "timeout_seconds", "max_retries", "notifications",
    "schedule_type", "run_at", "delay_seconds",
]


def token():
    return subprocess.check_output(
        ["docker", "exec", "unimap-unimap-1", "printenv", "UNIMAP_ADMIN_TOKEN"],
        text=True,
    ).strip()


TOKEN = token()


def api(path, body=None, method=None):
    req = urllib.request.Request(
        API + path,
        data=json.dumps(body).encode() if body is not None else None,
        method=method,
        headers={
            "X-Admin-Token": TOKEN,
            "Origin": "http://127.0.0.1:8448",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req) as r:
            raw = r.read().decode()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        return {"_http_error": e.code, "_body": e.read().decode()[:300]}


def main():
    for name, tid, q, cron in UPDATE:
        t = api("/get?id=" + tid)
        if not t or "id" not in t:
            print("WARN: %s not found" % name)
            continue
        t["cron_expr"] = cron
        t["payload"] = {
            "query": q,
            "engines": ["hunter"],
            "page_size": 100,
            "format": "excel",
            "only_new": True,
            "notification_detail_limit": 100,
        }
        body = {k: t[k] for k in ALLOWED if k in t}
        print("=== update %s -> cron=%s" % (name, cron))
        print("  resp:", api("/update", body=body, method="POST"))

    for name, tid in DELETE:
        print("=== delete %s" % name)
        print("  resp:", api("/delete", body={"id": tid}, method="POST"))


if __name__ == "__main__":
    main()
