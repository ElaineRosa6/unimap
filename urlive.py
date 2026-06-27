#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import socket
import struct
import concurrent.futures
import pandas as pd
import re
import time
import threading
import random
import sys
import io
import http.client
import ssl
import hashlib
from urllib.parse import urlparse
from datetime import datetime

# ================= 修复终端中文乱码 =================
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')
# ====================================================

# ================= 配置区域 =================
#  扫描模式（取消注释对应行切换）：
#
#  激进模式 —— 最快，有防火墙drop的环境可能漏报
#    PORT_TIMEOUT = 0.2
#    PORT_THREAD_COUNT = 2000
#
#  均衡模式（默认推荐）—— 兼顾速度与准确率
#    PORT_TIMEOUT = 0.3
#    PORT_THREAD_COUNT = 1500
#
#  稳健模式 —— 不漏报，速度慢
#    PORT_TIMEOUT = 0.8
#    PORT_THREAD_COUNT = 800

PORT_TIMEOUT = 0.5
PORT_THREAD_COUNT = 1500

REQUEST_TIMEOUT = 5
START_PORT = 1
END_PORT = 65535
MAX_BODY_SIZE = 1048576
MAX_REDIRECTS = 5
# ===========================================

# UA池
USER_AGENTS = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 OPR/105.0.0.0",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
    "Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.2210.91",
]

# ================= 增强版指纹规则库 (100+ 条) =================
FOFA_RULES = [
    # -------------------- Web 服务器 --------------------
    {"name": "Nginx",       "rule": {"type": "header", "key": "Server",       "regex": r"nginx"}},
    {"name": "Apache",      "rule": {"type": "header", "key": "Server",       "regex": r"apache"}},
    {"name": "IIS",         "rule": {"type": "header", "key": "Server",       "regex": r"microsoft-iis|iis"}},
    {"name": "Tomcat",      "rule": {"type": "header", "key": "Server",       "regex": r"tomcat|coyote|Apache-Coyote"}},
    {"name": "OpenResty",   "rule": {"type": "header", "key": "Server",       "regex": r"openresty"}},
    {"name": "LiteSpeed",   "rule": {"type": "header", "key": "Server",       "regex": r"litespeed"}},
    {"name": "Tengine",     "rule": {"type": "header", "key": "Server",       "regex": r"tengine"}},
    {"name": "Caddy",       "rule": {"type": "header", "key": "Server",       "regex": r"caddy"}},
    {"name": "Kong",        "rule": {"type": "header", "key": "Server",       "regex": r"kong"}},
    {"name": "Envoy",       "rule": {"type": "header", "key": "Server",       "regex": r"envoy"}},
    {"name": "Wordpres Engine", "rule": {"type": "header", "key": "X-Pingback", "regex": r"wordpress"}},
    {"name": "Python",      "rule": {"type": "header", "key": "Server",       "regex": r"python|gunicorn|uvicorn|werkzeug"}},

    # -------------------- 开发语言 / 框架头 --------------------
    {"name": "PHP",         "rule": {"type": "header", "key": "X-Powered-By", "regex": r"php"}},
    {"name": "ASP.NET",     "rule": {"type": "header", "key": "X-Powered-By", "regex": r"asp\.net"}},
    {"name": "ThinkPHP",    "rule": {"type": "header", "key": "X-Powered-By", "regex": r"thinkphp"}},
    {"name": "Express",     "rule": {"type": "header", "key": "X-Powered-By", "regex": r"express"}},
    {"name": "Next.js",     "rule": {"type": "header", "key": "X-Powered-By", "regex": r"next\.js"}},
    {"name": "Spring",      "rule": {"type": "header", "key": "X-Application-Context", "regex": r".*"}},
    {"name": "Jenkins",     "rule": {"type": "header", "key": "X-Jenkins",    "regex": r".*"}},
    {"name": "JSP",         "rule": {"type": "cookie", "key": None,           "regex": r"JSESSIONID"}},
    {"name": "PHPSESSID",   "rule": {"type": "cookie", "key": None,           "regex": r"PHPSESSID"}},
    {"name": "Laravel",     "rule": {"type": "cookie", "key": None,           "regex": r"laravel_session"}},
    {"name": "Django",      "rule": {"type": "cookie", "key": None,           "regex": r"csrftoken"}},
    {"name": "RubyOnRails", "rule": {"type": "cookie", "key": None,           "regex": r"_session_id"}},
    {"name": "Shiro",       "rule": {"type": "cookie", "key": None,           "regex": r"rememberMe=deleteMe|rememberMe=simple"}},

    # -------------------- 前端框架 --------------------
    {"name": "Vue.js",      "rule": {"type": "body",   "key": None,           "regex": r"vue\.js|vue\.min\.js|vue\.global\.js|data-v-"}},
    {"name": "React",       "rule": {"type": "body",   "key": None,           "regex": r"react\.min\.js|react-dom|data-reactroot|__NEXT_DATA__"}},
    {"name": "Angular",     "rule": {"type": "body",   "key": None,           "regex": r"ng-version|angular\.min\.js"}},
    {"name": "jQuery",      "rule": {"type": "body",   "key": None,           "regex": r"jquery[\.\-]?[\d\.]*\.js|jquery\.min\.js"}},
    {"name": "Bootstrap",   "rule": {"type": "body",   "key": None,           "regex": r"bootstrap[\.\-]?[\d\.]*\.(css|js)"}},
    {"name": "Layui",       "rule": {"type": "body",   "key": None,           "regex": r"layui\.(css|js)|class=\"layui"}},
    {"name": "ElementUI",   "rule": {"type": "body",   "key": None,           "regex": r"element-ui\.(css|js)"}},
    {"name": "AntDesign",   "rule": {"type": "body",   "key": None,           "regex": r"antd\.(css|js)|ant-design"}},
    {"name": "ECharts",     "rule": {"type": "body",   "key": None,           "regex": r"echarts\.(min\.)?js"}},
    {"name": "Highcharts",  "rule": {"type": "body",   "key": None,           "regex": r"highcharts\.(js|more\.js)"}},
    {"name": "DataTables",  "rule": {"type": "body",   "key": None,           "regex": r"dataTables\.(min\.)?js"}},
    {"name": "D3.js",       "rule": {"type": "body",   "key": None,           "regex": r"d3\.js|d3\.min\.js"}},
    {"name": "AmazeUI",     "rule": {"type": "body",   "key": None,           "regex": r"amazeui\.(min\.)?css"}},

    # -------------------- CMS 系统 --------------------
    {"name": "WordPress",   "rule": {"type": "body",   "key": None,           "regex": r"wp-content|wp-includes|wordpress"}},
    {"name": "Drupal",      "rule": {"type": "body",   "key": None,           "regex": r"drupal|sites/default/files"}},
    {"name": "Joomla",      "rule": {"type": "body",   "key": None,           "regex": r"joomla"}},
    {"name": "DedeCMS",     "rule": {"type": "body",   "key": None,           "regex": r"/dede/|powerby.*dedecms|dedeajax"}},
    {"name": "Discuz",      "rule": {"type": "body",   "key": None,           "regex": r"content=\"Discuz!|discuz_uid"}},
    {"name": "Typecho",     "rule": {"type": "body",   "key": None,           "regex": r"typecho|typecho\.js"}},
    {"name": "Ghost",       "rule": {"type": "body",   "key": None,           "regex": r"ghost-|content=\"Ghost"}},
    {"name": "Hexo",        "rule": {"type": "body",   "key": None,           "regex": r"hexo-theme|hexo-generator"}},
    {"name": "Hugo",        "rule": {"type": "body",   "key": None,           "regex": r"generated by hugo|Hugo"}},
    {"name": "MediaWiki",   "rule": {"type": "body",   "key": None,           "regex": r"mediawiki|powered by mediawiki"}},
    {"name": "PbootCMS",    "rule": {"type": "body",   "key": None,           "regex": r"pbootcms|powerby.*pboot"}},

    # -------------------- OA / 办公系统 --------------------
    {"name": "泛微OA",      "rule": {"type": "body",   "key": None,           "regex": r"weaver\.|/weaver/|js\/ecology8"}},
    {"name": "致远OA",      "rule": {"type": "body",   "key": None,           "regex": r"seeyon|/seeyon/|A8"}},
    {"name": "通达OA",      "rule": {"type": "body",   "key": None,           "regex": r"通达OA|td_oa|inc\/js\/td"}},
    {"name": "用友NC",      "rule": {"type": "body",   "key": None,           "regex": r"yonyou|/nccloud/|iufo"}},
    {"name": "帆软报表",    "rule": {"type": "body",   "key": None,           "regex": r"fineReport|WebReport|决策报表"}},
    {"name": "蓝凌OA",      "rule": {"type": "body",   "key": None,           "regex": r"landray|/sys/portal/"}},
    {"name": "万户OA",      "rule": {"type": "body",   "key": None,           "regex": r"万户|whir.*oa|com\.whir"}},
    {"name": "红帆OA",      "rule": {"type": "body",   "key": None,           "regex": r"ioffice|hongfan|iOffice\.net"}},
    {"name": "九思OA",      "rule": {"type": "body",   "key": None,           "regex": r"jiusi|jioa"}},
    {"name": "合强OA",      "rule": {"type": "body",   "key": None,           "regex": r"iWebOA|合强"}},

    # -------------------- 中间件 / 应用服务器 --------------------
    {"name": "SpringBoot",  "rule": {"type": "body",   "key": None,           "regex": r"whitelabel error page"}},
    {"name": "WebLogic",    "rule": {"type": "body",   "key": None,           "regex": r"weblogic|welcome to weblogic"}},
    {"name": "WebSphere",   "rule": {"type": "header", "key": "Server",       "regex": r"websphere"}},
    {"name": "JBoss",       "rule": {"type": "header", "key": "Server",       "regex": r"jboss|jbossweb"}},
    {"name": "WildFly",     "rule": {"type": "header", "key": "Server",       "regex": r"wildfly"}},
    {"name": "GlassFish",   "rule": {"type": "header", "key": "Server",       "regex": r"glassfish"}},
    {"name": "Jetty",       "rule": {"type": "header", "key": "Server",       "regex": r"jetty"}},

    # -------------------- 微服务 / 注册中心 --------------------
    {"name": "Nacos",       "rule": {"type": "title",  "key": None,           "regex": r"nacos"}},
    {"name": "Nacos",       "rule": {"type": "body",   "key": None,           "regex": r"nacos|console-ui"}},
    {"name": "XXL-JOB",     "rule": {"type": "title",  "key": None,           "regex": r"xxl-job"}},
    {"name": "XXL-JOB",     "rule": {"type": "body",   "key": None,           "regex": r"xxl-job|jobexecutor"}},
    {"name": "Kafka-Eagle", "rule": {"type": "title",  "key": None,           "regex": r"kafka eagle"}},
    {"name": "DubboAdmin",  "rule": {"type": "title",  "key": None,           "regex": r"dubbo.*admin"}},
    {"name": "Sentinel",    "rule": {"type": "title",  "key": None,           "regex": r"sentinel.*dashboard"}},
    {"name": "SkyWalking",  "rule": {"type": "body",   "key": None,           "regex": r"skywalking|ui\.topology"}},
    {"name": "Consul",      "rule": {"type": "body",   "key": None,           "regex": r"consul|v1/agent|/ui/"}},
    {"name": "ZooKeeper",   "rule": {"type": "body",   "key": None,           "regex": r"zookeeper|zkui|<title>ZooKeeper"}},

    # -------------------- API 文档 / 接口管理 --------------------
    {"name": "Swagger",     "rule": {"type": "body",   "key": None,           "regex": r"swagger-ui|swagger\.css|swagger-ui\.bundle"}},
    {"name": "Swagger",     "rule": {"type": "title",  "key": None,           "regex": r"swagger ui"}},
    {"name": "Kong Admin",  "rule": {"type": "body",   "key": None,           "regex": r"kong admin|kong manager"}},
    {"name": "YApi",        "rule": {"type": "body",   "key": None,           "regex": r"yapi|YMFE"}},
    {"name": "Rap2",        "rule": {"type": "body",   "key": None,           "regex": r"rap2|rap\.delivered"}},

    # -------------------- 监控 / 运维 / 面板 --------------------
    {"name": "Zabbix",      "rule": {"type": "title",  "key": None,           "regex": r"zabbix"}},
    {"name": "Zabbix",      "rule": {"type": "body",   "key": None,           "regex": r"zabbix\.js|zabbix\.css"}},
    {"name": "Grafana",     "rule": {"type": "title",  "key": None,           "regex": r"grafana"}},
    {"name": "Grafana",     "rule": {"type": "body",   "key": None,           "regex": r"grafana|grafana\.min\.js"}},
    {"name": "Kibana",      "rule": {"type": "body",   "key": None,           "regex": r"kibana|kbn-logo"}},
    {"name": "Prometheus",  "rule": {"type": "title",  "key": None,           "regex": r"prometheus"}},
    {"name": "Prometheus",  "rule": {"type": "body",   "key": None,           "regex": r"prometheus|graphite-target"}},
    {"name": "宝塔面板",    "rule": {"type": "body",   "key": None,           "regex": r"bt\.cn|宝塔|linux面板"}},
    {"name": "宝塔面板",    "rule": {"type": "title",  "key": None,           "regex": r"宝塔"}},
    {"name": "1Panel",      "rule": {"type": "body",   "key": None,           "regex": r"1panel|FIT2CLOUD"}},
    {"name": "Cockpit",     "rule": {"type": "title",  "key": None,           "regex": r"cockpit"}},
    {"name": "Portainer",   "rule": {"type": "title",  "key": None,           "regex": r"portainer"}},
    {"name": "Rancher",     "rule": {"type": "body",   "key": None,           "regex": r"rancher|global-nav"}},
    {"name": "Navidrome",   "rule": {"type": "body",   "key": None,           "regex": r"navidrome"}},

    # -------------------- 数据库管理 --------------------
    {"name": "phpMyAdmin",  "rule": {"type": "title",  "key": None,           "regex": r"phpMyAdmin"}},
    {"name": "phpMyAdmin",  "rule": {"type": "body",   "key": None,           "regex": r"phpmyadmin|pma_theme|pma_navigation"}},
    {"name": "Adminer",     "rule": {"type": "body",   "key": None,           "regex": r"adminer|Login - Adminer"}},
    {"name": "DBeaver",     "rule": {"type": "body",   "key": None,           "regex": r"dbeaver|cloudbeaver"}},

    # -------------------- 代码托管 / DevOps --------------------
    {"name": "GitLab",      "rule": {"type": "title",  "key": None,           "regex": r"gitlab"}},
    {"name": "GitLab",      "rule": {"type": "body",   "key": None,           "regex": r"gitlab|data-project-id"}},
    {"name": "Gitea",       "rule": {"type": "body",   "key": None,           "regex": r"Powered by Gitea|gitea\.css"}},
    {"name": "Gogs",        "rule": {"type": "body",   "key": None,           "regex": r"Powered by Gogs|gogs\.css"}},
    {"name": "Jenkins",     "rule": {"type": "body",   "key": None,           "regex": r"jenkins|jenkins-shell|jenkins-ci"}},
    {"name": "SonarQube",   "rule": {"type": "title",  "key": None,           "regex": r"sonarqube"}},
    {"name": "SonarQube",   "rule": {"type": "body",   "key": None,           "regex": r"sonarqube|sonar-css"}},
    {"name": "Harbor",      "rule": {"type": "title",  "key": None,           "regex": r"harbor"}},
    {"name": "Nexus",       "rule": {"type": "body",   "key": None,           "regex": r"nexus.*repository|nexus.*oss"}},
    {"name": "Jira",        "rule": {"type": "body",   "key": None,           "regex": r"atlassian.*jira|data-jira"}},

    # -------------------- 消息队列 --------------------
    {"name": "RabbitMQ",    "rule": {"type": "body",   "key": None,           "regex": r"rabbitmq_management|rabbitmq-web"}},
    {"name": "Kafka",       "rule": {"type": "body",   "key": None,           "regex": r"ksqlDB|kafka.*rest"}},
    {"name": "RocketMQ",    "rule": {"type": "body",   "key": None,           "regex": r"rocketmq-console|ops.*rocketmq"}},

    # -------------------- 项目管理 / 协作 --------------------
    {"name": "禅道",        "rule": {"type": "body",   "key": None,           "regex": r"zentao|禅道"}},
    {"name": "禅道",        "rule": {"type": "title",  "key": None,           "regex": r"禅道"}},
    {"name": "Redmine",     "rule": {"type": "title",  "key": None,           "regex": r"redmine"}},
    {"name": "Taiga",       "rule": {"type": "body",   "key": None,           "regex": r"taiga|taiga-front"}},
    {"name": "Focalboard",  "rule": {"type": "body",   "key": None,           "regex": r"focalboard"}},
    {"name": "Outline",     "rule": {"type": "body",   "key": None,           "regex": r"outline.*knowledge"}},

    # -------------------- WAF / 安全设备 --------------------
    {"name": "WAF",         "rule": {"type": "body",   "key": None,           "regex": r"firewall|拦截|非法请求|safeline|长亭|safe dog|安全狗|云锁|D盾"}},
    {"name": "CloudFlare",  "rule": {"type": "header", "key": "Server",       "regex": r"cloudflare"}},
    {"name": "CloudFlare",  "rule": {"type": "header", "key": "CF-RAY",       "regex": r".*"}},
    {"name": "Akamai",      "rule": {"type": "header", "key": "X-Akamai",     "regex": r".*"}},

    # -------------------- 其他业务系统 --------------------
    {"name": "Nextcloud",   "rule": {"type": "body",   "key": None,           "regex": r"nextcloud"}},
    {"name": "ownCloud",    "rule": {"type": "body",   "key": None,           "regex": r"owncloud"}},
    {"name": "MantisBT",    "rule": {"type": "body",   "key": None,           "regex": r"mantisbt|mantis bug tracker"}},
    {"name": "GLPI",        "rule": {"type": "body",   "key": None,           "regex": r"glpi|ticket.*tracking"}},
    {"name": "SpamAssassin","rule": {"type": "body",   "key": None,           "regex": r"spamassassin"}},
    {"name": "Roundcube",   "rule": {"type": "body",   "key": None,           "regex": r"roundcube"}},
    {"name": "Seafile",     "rule": {"type": "body",   "key": None,           "regex": r"seafile|seahub"}},
    {"name": "ONLYOFFICE",  "rule": {"type": "body",   "key": None,           "regex": r"onlyoffice|documentserver"}},
    {"name": "MinIO",       "rule": {"type": "body",   "key": None,           "regex": r"minio|MINIO_BROWSER|S3.*Gateway"}},
    {"name": "FastDFS",     "rule": {"type": "body",   "key": None,           "regex": r"fastdfs|fdfs_tracker"}},
    {"name": "Traefik",     "rule": {"type": "header", "key": "Server",       "regex": r"traefik"}},
    {"name": "Censys",      "rule": {"type": "header", "key": "Server",       "regex": r"censys"}},
    {"name": "F5 BIG-IP",   "rule": {"type": "header", "key": "Server",       "regex": r"bigip|big-ip"}},
    {"name": "PaloAlto",    "rule": {"type": "header", "key": "Server",       "regex": r"globalprotect|paloalto"}},
    {"name": "Fortinet",    "rule": {"type": "header", "key": "Server",       "regex": r"fortinet|fortigate"}},
    {"name": "Microsoft ARR", "rule": {"type": "header", "key": "Server",     "regex": r"arr|application request routing"}},
    {"name": "Varnish",     "rule": {"type": "header", "key": "Server",       "regex": r"varnish"}},
    {"name": "Squid",       "rule": {"type": "header", "key": "Server",       "regex": r"squid"}},
    {"name": "HAProxy",     "rule": {"type": "header", "key": "Server",       "regex": r"haproxy"}},
    {"name": "Apache Shenyu","rule": {"type": "body",   "key": None,           "regex": r"apache shenyu|shenyu.*gateway"}},
    {"name": "JeecgBoot",   "rule": {"type": "body",   "key": None,           "regex": r"jeecg|jeecg-boot|jeecg\.css"}},
    {"name": "RuoYi",       "rule": {"type": "body",   "key": None,           "regex": r"ruoyi|若依"}},
    {"name": "Halo",        "rule": {"type": "body",   "key": None,           "regex": r"halo.*blog|halo-admin"}},
    {"name": "Docker Registry","rule": {"type": "body", "key": None,         "regex": r"docker-registry|v2/_catalog"}},
    {"name": "Vault",       "rule": {"type": "title",  "key": None,           "regex": r"vault"}},
    {"name": "Graylog",     "rule": {"type": "body",   "key": None,           "regex": r"graylog|graylog-web"}},
    {"name": "GoCD",        "rule": {"type": "title",  "key": None,           "regex": r"gocd"}},
    {"name": "JumpServer",  "rule": {"type": "body",   "key": None,           "regex": r"jumpserver|jumpserver.*assets"}},
    {"name": "Wiki.js",     "rule": {"type": "body",   "key": None,           "regex": r"wiki\.js|wikijs"}},
    {"name": "BookStack",   "rule": {"type": "body",   "key": None,           "regex": r"bookstack"}},
    {"name": "Edusoho",     "rule": {"type": "body",   "key": None,           "regex": r"edusoho|es-"}},
    {"name": "Teambition",  "rule": {"type": "body",   "key": None,           "regex": r"teambition"}},
    {"name": "Confluence",  "rule": {"type": "body",   "key": None,           "regex": r"confluence|atlassian.*confluence"}},
]


class AssetScanner:
    def __init__(self):
        self.print_lock = threading.Lock()
        self._scan_start = 0

    def get_random_headers(self):
        return {
            'User-Agent': random.choice(USER_AGENTS),
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
            'Connection': 'close'
        }

    def parse_url(self, url):
        if not url.startswith(('http://', 'https://')):
            url = 'http://' + url
        parsed = urlparse(url)
        host = parsed.hostname
        port = parsed.port
        if port is None:
            port = 443 if parsed.scheme == 'https' else 80
        return host, port, url

    def _build_http_connection(self, host, port, use_ssl):
        if use_ssl:
            ctx = ssl.create_default_context()
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE
            return http.client.HTTPSConnection(host, port, timeout=REQUEST_TIMEOUT, context=ctx)
        else:
            return http.client.HTTPConnection(host, port, timeout=REQUEST_TIMEOUT)

    # ================================================================
    #  存活探测 + 指纹计算
    #  MD5 = md5(状态行 + 完整响应头 + 空行 + 响应体)
    # ================================================================
    def check_survival_and_fingerprint(self, url):
        host, business_port, full_url = self.parse_url(url)
        is_alive = False
        md5_fingerprint = ""
        regex_fingerprints = []
        current_url = full_url

        try:
            for _ in range(MAX_REDIRECTS):
                c_parsed = urlparse(current_url)
                c_host = c_parsed.hostname
                c_port = c_parsed.port or (443 if c_parsed.scheme == 'https' else 80)
                c_path = c_parsed.path or '/'
                c_ssl = c_parsed.scheme == 'https'

                conn = self._build_http_connection(c_host, c_port, c_ssl)
                conn.request("GET", c_path, headers=self.get_random_headers())
                resp = conn.getresponse()
                is_alive = True

                # 跟随重定向
                if resp.status in (301, 302, 303, 307, 308):
                    location = resp.getheader('Location')
                    conn.close()
                    if not location:
                        break
                    if location.startswith('/'):
                        scheme = 'https' if c_ssl else 'http'
                        current_url = f"{scheme}://{c_host}:{c_port}{location}"
                    else:
                        current_url = location
                    continue

                # ---- 拼接原始HTTP响应头 ----
                version_str = "HTTP/1.1" if resp.version == 11 else "HTTP/1.0"
                reason = resp.reason or ""
                status_line = f"{version_str} {resp.status} {reason}\r\n"

                header_lines = ""
                resp_headers_dict = {}
                for h_name, h_value in resp.getheaders():
                    header_lines += f"{h_name}: {h_value}\r\n"
                    if h_name in resp_headers_dict:
                        if isinstance(resp_headers_dict[h_name], list):
                            resp_headers_dict[h_name].append(h_value)
                        else:
                            resp_headers_dict[h_name] = [resp_headers_dict[h_name], h_value]
                    else:
                        resp_headers_dict[h_name] = h_value

                body = resp.read(MAX_BODY_SIZE)
                conn.close()

                # ---- MD5指纹 ----
                raw_http_bytes = (status_line + header_lines).encode('utf-8', errors='ignore') + b"\r\n" + body
                md5_fingerprint = hashlib.md5(raw_http_bytes).hexdigest()

                # ---- 正则指纹 ----
                body_text = body.decode('utf-8', errors='ignore').lower()

                title = ""
                title_match = re.search(r"<title>(.*?)</title>", body_text, re.IGNORECASE | re.DOTALL)
                if title_match:
                    title = title_match.group(1).strip()

                set_cookie_raw = resp_headers_dict.get('Set-Cookie', '')

                for rule_item in FOFA_RULES:
                    rule = rule_item['rule']
                    rtype = rule['type']
                    matched = False

                    if rtype == 'header':
                        key = rule.get('key')
                        if key and key in resp_headers_dict:
                            val = resp_headers_dict[key]
                            if isinstance(val, list):
                                val = ' '.join(val)
                            if re.search(rule['regex'], str(val), re.IGNORECASE):
                                matched = True

                    elif rtype == 'cookie':
                        if re.search(rule['regex'], str(set_cookie_raw), re.IGNORECASE):
                            matched = True

                    elif rtype == 'body':
                        if re.search(rule['regex'], body_text, re.IGNORECASE):
                            matched = True

                    elif rtype == 'title':
                        if title and re.search(rule['regex'], title, re.IGNORECASE):
                            matched = True

                    if matched:
                        regex_fingerprints.append(rule_item['name'])

                break

        except Exception:
            pass

        return is_alive, business_port, md5_fingerprint, sorted(list(set(regex_fingerprints)))

    # ================================================================
    #  端口扫描 (SO_LINGER + 分批提交 + 安全关闭)
    # ================================================================
    def scan_port(self, host, port):
        s = None
        try:
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s.settimeout(PORT_TIMEOUT)
            # 发RST包立即释放FD，不再进入TIME_WAIT
            s.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack('ii', 1, 0))
            result = s.connect_ex((host, port))
            if result == 0:
                return port, True
            return port, False
        except Exception:
            return port, False
        finally:
            if s is not None:
                try:
                    s.close()
                except Exception:
                    pass

    def scan_ports_threaded(self, host, url_index, total_urls):
        open_ports = []
        total_ports = END_PORT - START_PORT + 1
        scanned_count = 0
        batch_size = 5000
        all_ports = range(START_PORT, END_PORT + 1)
        port_batches = [all_ports[i:i + batch_size] for i in range(0, total_ports, batch_size)]

        with concurrent.futures.ThreadPoolExecutor(max_workers=PORT_THREAD_COUNT) as executor:
            for batch in port_batches:
                future_to_port = {executor.submit(self.scan_port, host, p): p for p in batch}
                for future in concurrent.futures.as_completed(future_to_port):
                    try:
                        _, is_open = future.result()
                    except Exception:
                        is_open = False

                    if is_open:
                        open_ports.append(future_to_port[future])

                    with self.print_lock:
                        scanned_count += 1
                        if scanned_count % 1000 == 0 or scanned_count == total_ports:
                            pct = (scanned_count / total_ports) * 100
                            elapsed = time.time() - self._scan_start
                            if elapsed > 0:
                                rate = scanned_count / elapsed
                                remain = (total_ports - scanned_count) / rate
                                eta_str = f"{remain:.0f}s" if remain < 60 else f"{remain/60:.1f}min"
                            else:
                                eta_str = "计算中..."
                            print(f"\r  └─ 端口 {scanned_count}/{total_ports} ({pct:.1f}%) | 发现 {len(open_ports)} 开放 | 剩余 {eta_str}   ", end="")

        print()
        open_ports.sort()
        return open_ports


def main(input_file, output_file):
    scanner = AssetScanner()

    try:
        with open(input_file, 'r', encoding='utf-8') as f:
            urls = [line.strip() for line in f if line.strip()]
    except FileNotFoundError:
        print(f"[!] 输入文件 {input_file} 未找到。")
        return

    if not urls:
        print("[!] 输入文件为空。")
        return

    results = []
    total = len(urls)
    total_ports = END_PORT - START_PORT + 1

    print("=" * 65)
    print("  资产存活探测 & 全端口扫描 & 指纹识别 v3.0")
    print(f"  指纹算法: MD5(完整HTTP响应头 + 空行 + HTTP响应体)")
    print(f"  指纹规则: {len(FOFA_RULES)} 条 (Header/Body/Title/Cookie)")
    print(f"  端口范围: {START_PORT}-{END_PORT} | 超时: {PORT_TIMEOUT}s | 线程: {PORT_THREAD_COUNT}")
    print(f"  理论最坏(单目标): ~{total_ports/PORT_THREAD_COUNT*PORT_TIMEOUT:.0f}s")
    print(f"  目标数量: {total}")
    print("=" * 65)

    for index, url in enumerate(urls, 1):
        host, _, _ = scanner.parse_url(url)
        print(f"\n[{index}/{total}] {url}")

        # 1. 存活 + 指纹
        is_alive, biz_port, md5_fp, regex_fps = scanner.check_survival_and_fingerprint(url)
        if is_alive:
            print(f"  └─ 存活: 是 | 端口: {biz_port}")
            print(f"  └─ MD5指纹: {md5_fp}")
            if regex_fps:
                print(f"  └─ 识别组件: {', '.join(regex_fps)}")
        else:
            print(f"  └─ 存活: 否")

        # 2. 端口扫描
        open_ports = []
        if host:
            scanner._scan_start = time.time()
            open_ports = scanner.scan_ports_threaded(host, index, total)

        # 3. 汇总
        fp_display = md5_fp if is_alive else "不存活"
        if is_alive and regex_fps:
            fp_display += f" [{', '.join(regex_fps)}]"

        results.append({
            "URL": url,
            "URL对应业务端口": biz_port,
            "是否存活": "存活" if is_alive else "不存活",
            "开放端口": ", ".join(map(str, open_ports)) if open_ports else "无",
            "资产指纹": fp_display
        })

    # 导出Excel
    print("\n" + "=" * 65)
    print("[+] 扫描完成，正在导出Excel...")
    df = pd.DataFrame(results)
    try:
        df.to_excel(output_file, index=False, engine='openpyxl')
        print(f"[+] 已保存: {output_file}")
    except Exception as e:
        print(f"[!] 导出失败: {e}")


if __name__ == "__main__":
    INPUT_TXT = "urls.txt"
    OUTPUT_EXCEL = f"scan_result_{datetime.now().strftime('%Y%m%d_%H%M%S')}.xlsx"

    start_time = time.time()
    main(INPUT_TXT, OUTPUT_EXCEL)
    print(f"[*] 总耗时: {time.time() - start_time:.2f} 秒")

