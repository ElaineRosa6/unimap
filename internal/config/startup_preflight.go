package config

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// IsLoopbackBind 判断绑定地址是否仅监听回环接口。
// 处理空白、localhost 大小写（含尾点）、IPv4 127.0.0.0/8 与 ::1；
// 0.0.0.0 绑定所有接口（Docker/云部署常见），不视为回环。
func IsLoopbackBind(addr string) bool {
	host := strings.TrimSpace(addr)
	if host == "" {
		return false
	}
	// 去除 host:port 或 [::1]:port 的端口部分
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") || strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return ip.IsLoopback()
}

// StartupPreflight 校验 Web 服务启动前的认证配置。
// 仅由 Web 主入口（cmd/unimap-web）在服务创建前调用；CLI/GUI 不调用，
// 因此不会被 Web 的非 loopback 暴露策略意外阻断。
// 本函数只返回错误，绝不调用 logger.Fatal/Fatalf，保证可单测。
//
// 契约：
//   - loopback 绑定：允许 admin_token / password_hash 为空，由现有“首用户注册”流程初始化；
//     不恢复 admin/admin 默认口令，也不自动生成不可获知的 admin_token。
//   - 非 loopback 绑定（如 0.0.0.0 / Docker / 云部署）：至少要求
//     web.auth.enabled=true、web.auth.admin_token 显式非空、
//     web.auth.username 显式非空且不等于 "admin"、web.auth.password_hash 为有效 bcrypt。
//     错误信息逐项指出缺失/无效字段。
func StartupPreflight(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("web auth startup preflight: config is nil")
	}
	bind := strings.TrimSpace(cfg.Web.BindAddress)
	if IsLoopbackBind(bind) {
		return nil
	}

	var missing []string
	if !cfg.Web.Auth.Enabled {
		missing = append(missing, "web.auth.enabled=true")
	}
	if strings.TrimSpace(cfg.Web.Auth.AdminToken) == "" {
		missing = append(missing, "web.auth.admin_token (explicit non-empty)")
	}
	username := strings.TrimSpace(cfg.Web.Auth.Username)
	switch {
	case username == "":
		missing = append(missing, "web.auth.username (explicit non-empty)")
	case strings.EqualFold(username, "admin"):
		missing = append(missing, `web.auth.username (must not be "admin")`)
	}
	if !isValidBcryptHash(cfg.Web.Auth.PasswordHash) {
		missing = append(missing, "web.auth.password_hash (valid bcrypt)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("web auth startup preflight failed for non-loopback bind %q: missing/invalid: %s",
			bind, strings.Join(missing, ", "))
	}
	return nil
}

// isValidBcryptHash 报告 s 是否为可用的 bcrypt 哈希。
func isValidBcryptHash(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	_, err := bcrypt.Cost([]byte(s))
	return err == nil
}
