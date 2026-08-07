#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""UniMap webhook -> SMTP email relay.

Receives UniMap task notifications (TaskNotification JSON) on a webhook
endpoint and forwards them to a target mailbox over SMTP.

Nothing is hard-coded: every parameter comes from the environment.

Environment variables (SMTP_USER / SMTP_PASSWORD / MAIL_TO are required):
    SMTP_HOST            SMTP server host            (default smtp.qq.com)
    SMTP_PORT            SMTP port                   (default 465, implicit SSL)
    SMTP_USER            sender mailbox              (required)
    SMTP_PASSWORD        SMTP credential             (required; for QQ this is
                         the SMTP authorization code, not the login password)
    MAIL_TO              recipient mailbox(es), comma-separated (required)
    MAIL_FROM_NAME       sender display name         (default "UniMap")
    MAIL_SUBJECT_PREFIX  subject prefix              (default "[UniMap]")
    RELAY_LISTEN         listen host:port            (default 0.0.0.0:8099)
    RELAY_TOKEN          optional bearer token; when set it must be presented
                         in the X-Relay-Token header of every POST /webhook.

HTTP endpoints:
    GET  /health         -> 200 {"status":"ok"} (liveness probe)
    POST /webhook        -> accepts TaskNotification JSON and relays it as email
"""

import json
import logging
import os
import re
import smtplib
import sys
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from email.utils import formataddr, formatdate
from html import escape
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

LOG = logging.getLogger("unimap-smtp-relay")

STATUS_ZH = {"success": "成功", "failed": "失败", "timeout": "超时"}


def _env(name, default=None):
    val = os.environ.get(name)
    if val is None:
        return default
    val = val.strip()
    return val if val else default


def load_config():
    cfg = {
        "smtp_host": _env("SMTP_HOST", "smtp.qq.com"),
        "smtp_port": int(_env("SMTP_PORT", "465")),
        "smtp_user": _env("SMTP_USER"),
        "smtp_password": _env("SMTP_PASSWORD"),
        "mail_to": [x.strip() for x in (_env("MAIL_TO", "") or "").split(",") if x.strip()],
        "mail_from_name": _env("MAIL_FROM_NAME", "UniMap"),
        "subject_prefix": _env("MAIL_SUBJECT_PREFIX", "[UniMap]"),
        "listen": _env("RELAY_LISTEN", "0.0.0.0:8099"),
        "token": _env("RELAY_TOKEN"),
    }
    missing = [k for k in ("smtp_user", "smtp_password") if not cfg[k]]
    if missing:
        raise RuntimeError("missing required env var(s): " + ", ".join(missing))
    if not cfg["mail_to"]:
        raise RuntimeError("missing required env var: MAIL_TO")
    return cfg


# --- minimal markdown -> html, enough for UniMap task result blocks ------------

_BOLD = re.compile(r"\*\*(.+?)\*\*")
_CODE = re.compile(r"`([^`]+)`")
_URL = re.compile(r"(https?://[^\s<]+)")
_TABLE_SEP = re.compile(r"^\|[\s:\-|]+\|\s*$")


def _inline_html(text):
    text = escape(text)
    text = _CODE.sub(lambda m: "<code>%s</code>" % m.group(1), text)
    text = _BOLD.sub(lambda m: "<strong>%s</strong>" % m.group(1), text)
    text = _URL.sub(lambda m: '<a href="%s">%s</a>' % (m.group(1), m.group(1)), text)
    return text


def _md_to_html(text):
    lines = text.splitlines()
    out = []
    i = 0
    while i < len(lines):
        line = lines[i].rstrip()
        if not line.strip():
            i += 1
            continue
        # Consecutive table rows (| a | b | ...)
        if line.lstrip().startswith("|") and line.count("|") >= 2:
            rows = []
            while i < len(lines) and lines[i].strip().count("|") >= 2:
                if not _TABLE_SEP.match(lines[i].strip()):
                    rows.append(lines[i].strip())
                i += 1
            if rows:
                table = ["<table>"]
                for idx, row in enumerate(rows):
                    cells = [_inline_html(c.strip()) for c in row.strip().strip("|").split("|")]
                    tag = "th" if idx == 0 else "td"
                    table.append("<tr>" + "".join("<%s>%s</%s>" % (tag, c, tag) for c in cells) + "</tr>")
                table.append("</table>")
                out.append("".join(table))
            continue
        # Standalone bold line -> heading
        if line.startswith("**") and line.endswith("**"):
            out.append("<h3>%s</h3>" % escape(line[2:-2]))
            i += 1
            continue
        out.append("<p>%s</p>" % _inline_html(line))
        i += 1
    return "".join(out)


# --- message construction ------------------------------------------------------

def _build_message(cfg, n):
    status = n.get("status", "?")
    status_zh = STATUS_ZH.get(status, status)
    task_name = n.get("task_name", "(unknown)")
    subject = "%s %s %s" % (cfg["subject_prefix"], status_zh, task_name)

    dur = n.get("duration_ms", 0)
    dur = "%d ms" % dur if isinstance(dur, (int, float)) else str(dur)
    ts = str(n.get("timestamp", ""))

    payload = n.get("payload") or {}
    query = payload.get("query", "")
    engines = payload.get("engines") or []
    if isinstance(engines, list):
        engines = ", ".join(str(e) for e in engines)
    else:
        engines = str(engines)

    plain = [
        "任务: %s (%s)" % (task_name, n.get("task_type", "")),
        "状态: %s (%s)" % (status_zh, status),
        "时间: %s" % ts,
        "耗时: %s" % dur,
    ]
    html = [
        "<p><b>任务:</b> %s (%s)</p>" % (escape(task_name), escape(str(n.get("task_type", "")))),
        "<p><b>状态:</b> %s (%s)</p>" % (escape(status_zh), escape(status)),
        "<p><b>时间:</b> %s</p>" % escape(ts),
        "<p><b>耗时:</b> %s</p>" % escape(dur),
    ]
    if query:
        plain.append("查询: %s" % query)
        html.append("<p><b>查询:</b> %s</p>" % escape(query))
    if engines:
        plain.append("引擎: %s" % engines)
        html.append("<p><b>引擎:</b> %s</p>" % escape(engines))
    if n.get("error"):
        plain.append("错误: %s" % n["error"])
        html.append('<p><b>错误:</b> <span style="color:#c0392b">%s</span></p>' % escape(str(n["error"])))
    if n.get("result"):
        plain.append("")
        plain.append(n["result"])
        html.append(_md_to_html(str(n["result"])))

    msg = MIMEMultipart("alternative")
    msg["Subject"] = subject
    msg["From"] = formataddr((cfg["mail_from_name"], cfg["smtp_user"]))
    msg["To"] = ", ".join(cfg["mail_to"])
    msg["Date"] = formatdate(localtime=True)
    msg.attach(MIMEText("\n".join(plain), "plain", "utf-8"))
    msg.attach(MIMEText("".join(html), "html", "utf-8"))
    return msg


def _send(cfg, n):
    msg = _build_message(cfg, n)
    if cfg["smtp_port"] == 465:
        with smtplib.SMTP_SSL(cfg["smtp_host"], cfg["smtp_port"], timeout=30) as smtp:
            smtp.login(cfg["smtp_user"], cfg["smtp_password"])
            smtp.sendmail(cfg["smtp_user"], cfg["mail_to"], msg.as_string())
    else:
        with smtplib.SMTP(cfg["smtp_host"], cfg["smtp_port"], timeout=30) as smtp:
            smtp.starttls()
            smtp.login(cfg["smtp_user"], cfg["smtp_password"])
            smtp.sendmail(cfg["smtp_user"], cfg["mail_to"], msg.as_string())


# --- HTTP server -----------------------------------------------------------------

class RelayHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _json(self, code, obj):
        body = json.dumps(obj, ensure_ascii=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.rstrip("/") == "/health":
            self._json(200, {"status": "ok"})
        else:
            self._json(404, {"error": "not found"})

    def do_POST(self):
        if self.path.rstrip("/") != "/webhook":
            self._json(404, {"error": "not found"})
            return
        token = self.server.cfg["token"]
        if token and self.headers.get("X-Relay-Token", "") != token:
            self._json(401, {"error": "unauthorized"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self._json(400, {"error": "invalid content-length"})
            return
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            n = json.loads(raw.decode("utf-8"))
        except Exception as exc:
            LOG.warning("bad webhook payload: %s", exc)
            self._json(400, {"error": "invalid json"})
            return
        try:
            _send(self.server.cfg, n)
        except Exception as exc:
            LOG.error("smtp send failed: %s", exc)
            self._json(502, {"error": "smtp send failed: %s" % exc})
            return
        LOG.info("relayed notification task=%s status=%s", n.get("task_name"), n.get("status"))
        self._json(200, {"status": "ok"})

    def log_message(self, fmt, *args):
        LOG.info("%s - %s", self.address_string(), fmt % args)


def main():
    logging.basicConfig(
        level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s"
    )
    try:
        cfg = load_config()
    except RuntimeError as exc:
        LOG.error("config error: %s", exc)
        sys.exit(1)
    host, _, port = cfg["listen"].rpartition(":")
    server = ThreadingHTTPServer((host, int(port)), RelayHandler)
    server.cfg = cfg
    LOG.info("relay listening on %s (POST /webhook)", cfg["listen"])
    LOG.info("smtp %s:%s user=%s to=%s", cfg["smtp_host"], cfg["smtp_port"],
             cfg["smtp_user"], ",".join(cfg["mail_to"]))
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
