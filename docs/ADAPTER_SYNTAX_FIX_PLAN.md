# 适配器语法翻译修复计划

> 基准：`docs/SEARCH_ENGINE_SYNTAX.md`
> 目标：让 `internal/adapter/{hunter,quake,shodan,zoomeye}.go` 的 `Translate` 输出与各引擎真实语法一致。
> 现状：5 引擎中仅 FOFA 正确；现有单测把错误输出固化为断言（绿测 ≠ 正确），修复须同步改测试。
> 验证命令：`go test -race ./internal/adapter/...`

## 严重度与影响概览

| 引擎 | 严重度 | 影响 | 修复项 |
|------|--------|------|--------|
| ZoomEye | 🔴 致命 | v1/v2 语法不匹配 + 双 `+`，查询基本全废 | FIX-Z1~Z4 |
| Shodan | 🟠 高 | 含空格值被拆、字段名错、布尔模型不符 | FIX-S1~S4 |
| Hunter | 🟠 高 | 多条件用 `AND`/`OR` 词、`header.server`/`ip.os` 错 | FIX-H1~H3 |
| Quake | 🟡 低 | `server→app` 语义错 | FIX-Q1 |
| Parser(公共) | 🟡 低 | 比较操作符静默降级、值引号未转义、死代码 | FIX-P1~P3 |

执行建议顺序：**ZoomEye → Shodan → Hunter → Quake → Parser**，每项独立提交、独立改测试、独立跑测试。

---

## ZoomEye

### ✅ FIX-Z1　分隔符 `:` → `=`，去掉 `+` 前缀（致命）
- 文件：`zoomeye.go:126-134` `buildCondition`
- 现状：`+field:"value"` / `-field:"value"`（v1 ES 语法）
- 目标：`field="value"`（模糊）、`field!="value"`（NOT）。`==` 精确保留为 `field=="value"`。
- 改法：移除 `prefix` 的 `+`/`-` 逻辑，改为
  - `=`/`==`/CONTAINS → `field="value"`（`==` 输出 `field=="value"`）
  - `!=`/`<>` → `field!="value"`

### ✅ FIX-Z2　逻辑连接符改 `&&` / `||`，消除双 `+`（致命）
- 文件：`zoomeye.go:82-91` `translateNode`
- 现状：AND → `+%s +%s`（叠加 Z1 的 `+` 形成 `++`）；OR → 空格
- 目标：AND → `(%s && %s)`；OR → `(%s || %s)`
- 依赖：与 FIX-Z1 同步改（Z1 去掉 `+` 后此处才正确）

### ✅ FIX-Z3　IN 语法修正
- 文件：`zoomeye.go:70-77`
- 现状：`(v1 v2 v3)`（空格连接，配合旧 `+` 前缀）
- 目标：`(field="v1" || field="v2" || field="v3")`

### ✅ FIX-Z4　字段映射对齐 v2
- 文件：`zoomeye.go:98-120`
- 修正：body→`http.body`、header→`http.header`、domain→`domain`、server→`http.header.server`、status_code→`http.header.status_code`；保留 title/port/service/ip/country/subdivisions/city/asn/org/isp/hostname/app/os/device/banner。

---

## Shodan

### ✅ FIX-S1　值加引号（高）
- 文件：`shodan.go:75,84,90` `translateNode`
- 现状：`field:value`（永不加引号）→ 含空格值如 `org:Beijing University` 被 Shodan 拆成 `org:Beijing` AND `University`
- 目标：值含空格（或特殊字符）时包裹引号 `field:"Beijing University"`；纯数字/无空格可不加。
- 建议：新增 helper `shodanQuote(v string) string`。

### ✅ FIX-S2　字段映射修正（高）
- 文件：`shodan.go:108-136` `mapField`
- 修正：title→`http.title`、body→`http.html`、status_code→`http.status`、host→`hostname`（单数）、app→`product`、cert→`ssl`；header/server 暂并入（见备注）。
- 备注：Shodan 无 `header`/`server` 独立 filter，header→`http.html`、server→`product` 为折中，需你确认是否接受。

### ✅ FIX-S3　NOT / 单字段 OR
- 文件：`shodan.go:86-88`、IN 分支 `:70-78`
- 现状：NOT → `-field:value` ✓（仅需补 S1 引号）；IN → `(f:a OR f:b)`
- 目标：IN 同字段改用逗号 `field:a,b`（Shodan 原生 OR），不再用 `(... OR ...)`。

### ✅ FIX-S4　布尔模型决策 — 静默降级 OR→AND + logger.Warnf
- 文件：`shodan.go:93-102` `translateNode` logical 分支
- 问题：Shodan 不支持 `()`、`AND`/`OR` 关键字、跨字段 OR。现状产 `(a AND b)`、`(a OR b)` 均非法。
- 方案 A（推荐·保守）：AND → 空格连接 `a b`（去括号去 AND 词）；跨字段 OR → 返回错误或降级提示"Shodan 不支持跨字段 OR"。
- 方案 B（尽力）：维持 `(a OR b)` 文本，接受结果不可靠。
- **决策点：A 还是 B？**

---

## Hunter

### ✅ FIX-H1　逻辑连接符 `AND`/`OR` → `&&`/`||`（高）
- 文件：`hunter.go:122-138` `translateNode`
- 现状：`(left AND right)` / `(left OR right)`（英文词）
- 目标：`(left && right)` / `(left || right)`

### ✅ FIX-H2　IN 连接符 ` OR ` → ` || `
- 文件：`hunter.go:110-117`
- 现状：`(a="x" OR b="y")`
- 目标：`(a="x" || b="y")`

### ✅ FIX-H3　字段映射修正
- 文件：`hunter.go:146-167` `buildCondition` mapping
- 修正：server→`header.server`（点号，非 `header_server`）、os→`ip.os`（现为 `os`）。
- 其余 web.title/web.body/ip.country/ip.province/ip.city/ip.asn/ip.org/ip.isp/app.name 经核对正确，保留。
- 待你核对：url→`web.url` 是否存在（官方页需登录）。

---

## Quake

### ✅ FIX-Q1　`server` 字段修正（低）
- 文件：`quake.go:130`
- 现状：server→`app`
- 目标：server→`server`
- 其余映射与 `field:"value"` / `NOT` / `AND`/`OR` 均正确，无需动。

---

## Parser 公共层

### ✅ FIX-P1　比较操作符智能解析（各引擎原生区间语法）
- 文件：`core/unimap/parser.go:274` 接受 `>`/`<`/`>=`/`<=`；各适配器 default 分支降级为等值
- 现状：`port>80` 被翻成 `port="80"`（hunter）等，静默错误
- 目标（二选一，需你拍板）：
  - A：各引擎对支持的字段输出原生区间（Quake `[81 TO *]`、ZoomEye `after`/Hunter `>`），不支持的返回错误。
  - B：暂不支持，遇比较操作符直接报错"该引擎不支持范围查询"，避免静默错误。
- **决策点：A（按引擎实现区间）还是 B（先报错）？** 建议先 B，A 作为后续迭代。

### ✅ FIX-P2　值内引号转义
- 文件：各适配器 `buildCondition`（重新包裹 `"%s"` 处）
- 现状：值含 `"` 直接拼接破坏查询（健壮性/轻度注入面）
- 目标：包裹前对值做转义（`"` → `\"`），按各引擎转义规则。

### ✅ FIX-P3　清理死代码
- 文件：`hunter.go:131-135` `node.Value == "NOT"` 分支（parser 不产生 NOT 逻辑节点）
- 目标：删除该分支注释块。

---

## 测试同步改动

每个 FIX 对应更新 `adapter_test.go` 中的 `want` 断言（当前锁定错误输出）：
- `TestHunterAdapter_Translate`：L512 `AND`→`&&`、L524 `OR`→`||`、server/os 字段。
- `TestShodanAdapter_Translate`：L779/791/815 改为引号 + 正确字段 + 布尔模型。
- ZoomEye：新增/重写 Translate 测试（现无独立 Translate 测试覆盖 v2 语法）。
- 补充 table-driven 用例覆盖：精确匹配 `==`、NOT、IN、嵌套 `()`、含空格值。

## 验收标准
- [ ] `go test -race ./internal/adapter/...` 全绿
- [ ] 每引擎至少 1 条手测查询经 Web 入口实际命中（用户侧验证）
- [ ] `go vet ./...` 无新增告警
- [ ] 文档 `SEARCH_ENGINE_SYNTAX.md` 与代码映射一致

## 待你确认的决策点汇总
1. **FIX-S4**：Shodan 跨字段 OR → 方案 A（报错/降级）还是 B（保留不可靠文本）？
2. **FIX-S2**：Shodan header→`http.html`、server→`product` 折中是否接受？
3. **FIX-P1**：比较操作符 → 方案 A（实现区间）还是 B（先报错）？
4. **FIX-H3**：Hunter `url→web.url` 是否需你登录官方页核对后再定？
5. **执行范围**：本轮全做（Z+S+H+Q+P），还是只做致命/高（Z+S+H）、Quake/Parser 下轮？
