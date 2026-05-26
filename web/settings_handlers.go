package web

import (
	"net/http"

	"github.com/unimap/project/internal/adapter"
)

// handleSettingsPage renders the unified settings page (GET /settings).
// Contains panels for: engines / ICP server / screenshot / cookies / session / system.
func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Pre-populate cookie fields for the cookies panel.
	cookieFofa := ""
	cookieHunter := ""
	cookieZoomeye := ""
	cookieQuake := ""
	proxyServer := ""
	if s.config != nil {
		s.configMutex.Lock()
		cookieFofa = cookiesToHeader(s.config.Engines.Fofa.Cookies)
		cookieHunter = cookiesToHeader(s.config.Engines.Hunter.Cookies)
		cookieZoomeye = cookiesToHeader(s.config.Engines.Zoomeye.Cookies)
		cookieQuake = cookiesToHeader(s.config.Engines.Quake.Cookies)
		proxyServer = s.config.Screenshot.ProxyServer
		s.configMutex.Unlock()
	}

	// ICP type enumeration for the ICP panel.
	icpTypes := make([]map[string]string, 0, len(adapter.AllICPQueryTypes()))
	for _, t := range adapter.AllICPQueryTypes() {
		icpTypes = append(icpTypes, map[string]string{
			"value": string(t),
			"label": adapter.ICPTypeLabel(t),
		})
	}

	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "settings.html", map[string]interface{}{
		"cookieFofa":    cookieFofa,
		"cookieHunter":  cookieHunter,
		"cookieZoomeye": cookieZoomeye,
		"cookieQuake":   cookieQuake,
		"proxyServer":   proxyServer,
		"icpTypes":      icpTypes,
		"staticVersion": s.staticVersion,
	}) {
		return
	}
}

