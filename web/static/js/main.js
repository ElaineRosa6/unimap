// 页面加载完成后执行
 document.addEventListener('DOMContentLoaded', function() {
	// 初始化WebSocket连接
	initWebSocket();

	// 初始化查询表单
	initQueryForm();

	// 初始化结果表格
	initResultsTable();

	// 初始化配额页面
	initQuotaPage();

	// 页面离开时清理所有定时器和连接，防止泄漏
	window.addEventListener('beforeunload', stopAllPolling);
});

// 初始化查询表单
function initQueryForm() {
	const form = document.querySelector('form[action="/query"]');
	if (!form) return;
	
	// 示例查询点击事件
	const examples = document.querySelectorAll('.example-item code');
	examples.forEach(example => {
		example.addEventListener('click', function() {
			const queryInput = document.getElementById('query');
			queryInput.value = this.textContent.trim();
			queryInput.focus();
		});
	});
	
	// 工具栏按钮事件
	const toolbarBtns = document.querySelectorAll('.toolbar-btn');
	toolbarBtns.forEach(btn => {
		btn.addEventListener('click', function() {
			const action = this.getAttribute('data-action');
			handleToolbarAction(action);
		});
	});
	
	// 保存查询按钮
	const saveQueryBtn = document.getElementById('btn-save-query');
	if (saveQueryBtn) {
		saveQueryBtn.addEventListener('click', function() {
			saveQuery();
		});
	}

	const saveCookiesBtn = document.getElementById('btn-save-cookies');
	if (saveCookiesBtn) {
		saveCookiesBtn.addEventListener('click', function() {
			saveCookies(saveCookiesBtn);
		});
	}

	const clearCookiesBtn = document.getElementById('btn-clear-cookies');
	if (clearCookiesBtn) {
		clearCookiesBtn.addEventListener('click', function() {
			clearCookies(clearCookiesBtn);
		});
	}

	const verifyCookiesBtn = document.getElementById('btn-verify-cookies');
	if (verifyCookiesBtn) {
		verifyCookiesBtn.addEventListener('click', function() {
			verifyCookies(verifyCookiesBtn);
		});
	}

	const importCookieBtn = document.getElementById('btn-import-cookie-json');
	if (importCookieBtn) {
		importCookieBtn.addEventListener('click', function() {
			importCookieJSON(importCookieBtn);
		});
	}

	// 登录状态刷新按钮
	const refreshLoginBtn = document.getElementById('btn-refresh-login-status');
	if (refreshLoginBtn) {
		refreshLoginBtn.addEventListener('click', function() {
			refreshLoginStatus();
		});
	}

	// Cookie 折叠按钮
	const toggleCookieBtn = document.getElementById('btn-toggle-cookie');
	if (toggleCookieBtn) {
		toggleCookieBtn.addEventListener('click', function() {
			toggleCookieSection();
		});
	}

	// 初始化登录状态轮询
	initLoginStatusPoll();

	initCookieStatus();
	initCDPControls();
	initBridgeStatusControls();
	initScreenshotModeSelector();
	
	// 表单提交事件
	form.addEventListener('submit', function(e) {
		e.preventDefault(); // 阻止默认提交
		
		const query = document.getElementById('query').value;
		if (!query.trim()) {
			alert('请输入查询语句');
			return;
		}
		
		// 保存到查询历史
		saveQueryToHistory(query);
		
		// 显示加载状态
		const submitBtn = form.querySelector('button[type="submit"]');
		const originalText = submitBtn.textContent;
		submitBtn.textContent = '查询中...';
		submitBtn.disabled = true;
		submitBtn.classList.add('loading');
		
		// 获取选中的引擎
		const engines = [];
		const engineInputs = document.querySelectorAll('input[name="engines"]:checked');
		engineInputs.forEach(input => {
			engines.push(input.value);
		});
		
		if (engines.length === 0) {
			alert('请至少选择一个引擎');
			submitBtn.textContent = originalText;
			submitBtn.disabled = false;
			submitBtn.classList.remove('loading');
			return;
		}

		// P1-3: 预查询校验 — 检查选中引擎是否配置了 API Key
		const enginesWithKey = engines.filter(e => engineStatusMap[e] && engineStatusMap[e].hasKey);
		if (enginesWithKey.length === 0) {
			alert('所选引擎均未配置 API Key，请先前往设置页面添加引擎密钥。');
			submitBtn.textContent = originalText;
			submitBtn.disabled = false;
			submitBtn.classList.remove('loading');
			return;
		}

		const browserQuery = isBrowserQueryModeEnabled();
		if (browserQuery && !isBrowserModeAvailable()) {
			const mode = getSelectedScreenshotMode();
			let msg = '浏览器查询模式当前不可用';
			if (mode === 'cdp') {
				msg = 'CDP 模式需要先连接 CDP 浏览器';
			} else if (mode === 'extension') {
				msg = '扩展模式需要扩展桥接在线';
			}
			alert(msg);
			submitBtn.textContent = originalText;
			submitBtn.disabled = false;
			submitBtn.classList.remove('loading');
			return;
		}
		
		// 执行异步查询
		const browserAction = getBrowserAction();
			executeAsyncQuery(query, engines, enginesWithKey, submitBtn, originalText, browserQuery, browserAction);
	});

	// 初始化引擎状态
	checkEngineStatus();
	// 检查登录状态
	checkLoginStatus();
}

function initCDPControls() {
	const statusBadge = document.getElementById('cdp-status');
	const statusInfo = document.getElementById('cdp-status-info');
	const connectBtn = document.getElementById('btn-connect-cdp');
	const saveProxyBtn = document.getElementById('btn-save-proxy');
	const miniIndicator = document.getElementById('cdp-status-mini');

	// P2-6: 即使没有完整控制面板，也要更新 mini 指示器
	if (!statusBadge && !statusInfo && !connectBtn && !miniIndicator) {
		return;
	}

	const refresh = function() {
		refreshCDPStatus(statusBadge, statusInfo);
	};

	refresh();
	if (cdpStatusTimer) clearInterval(cdpStatusTimer);
	cdpStatusTimer = setInterval(refresh, 15000);

	if (connectBtn) {
		connectBtn.addEventListener('click', function() {
			connectCDP(connectBtn, statusBadge, statusInfo);
		});
	}

	if (saveProxyBtn) {
		saveProxyBtn.addEventListener('click', function() {
			saveProxy(saveProxyBtn, statusInfo);
		});
	}
}

function initBridgeStatusControls() {
	const statusBadge = document.getElementById('bridge-status');
	const statusInfo = document.getElementById('bridge-status-info');
	const refreshBtn = document.getElementById('btn-refresh-bridge-status');

	const miniIndicator = document.getElementById('bridge-status-mini');
	if (!statusBadge && !statusInfo && !refreshBtn && !miniIndicator) {
		return;
	}

	const refresh = function() {
		refreshBridgeStatus(statusBadge, statusInfo);
	};

	refresh();
	if (bridgeStatusTimer) clearInterval(bridgeStatusTimer);
	bridgeStatusTimer = setInterval(refresh, 15000);

	if (refreshBtn) {
		refreshBtn.addEventListener('click', refresh);
	}
}

// Initialize the screenshot mode selector UI.
function initScreenshotModeSelector() {
	// Restore saved mode from localStorage
	const savedMode = localStorage.getItem('screenshotMode');
	if (savedMode) {
		const radio = document.querySelector(`input[name="screenshot-mode"][value="${savedMode}"]`);
		if (radio) radio.checked = true;
	}

	// Send mode change request when selection changes
	const radios = document.querySelectorAll('input[name="screenshot-mode"]');
	for (const radio of radios) {
		radio.addEventListener('change', function() {
			const mode = this.value;
			localStorage.setItem('screenshotMode', mode);
			apiFetch('/api/v1/screenshot/set-mode', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ mode })
			}).then(parseJsonResponse)
				.then(data => {
					console.log('Screenshot mode set to:', data.mode);
				})
				.catch(function(err) { console.warn('Screenshot mode init failed:', err); });
		});
	}
}

function refreshBridgeStatus(statusBadge, statusInfo) {
	apiFetch('/api/v1/screenshot/bridge/status')
		.then(parseJsonResponse)
		.then(data => {
			// Consume runtime state from ScreenshotRouter (authoritative)
			const routerMode = data && data.router_mode ? String(data.router_mode) : '';
			const cdpHealthy = !!(data && data.router_cdp_healthy);
			const extHealthy = !!(data && (data.extension_online || data.router_ext_healthy || (data.live_clients || 0) > 0));

			// Fallback to legacy config-level fields
			const engine = data && data.engine ? String(data.engine) : 'cdp';
			const extensionEnabled = !!(data && data.extension_enabled);
			const bridgeConnected = !!(data && data.bridge_connected);
			const pairedClients = Number((data && data.paired_clients) || 0);
			const liveClients = Number((data && data.live_clients) || 0);
			const pendingTasks = Number((data && data.pending_tasks) || 0);

			// Derive effective mode: prefer router runtime state, fall back to config
			const effectiveMode = routerMode || engine;
			const isConnected = effectiveMode === 'extension' && extHealthy;

			// Track bridge availability for browser_query gating
			bridgeOnline = effectiveMode === 'auto'
				? (cdpHealthy || extHealthy)
				: effectiveMode === 'extension'
					? extHealthy
					: effectiveMode === 'cdp'
						? cdpHealthy
						: liveClients > 0;

			// Sync extension login status to avoid "在线" vs "未配对" contradiction
			const extLoginEl = document.getElementById('status-ext');
			if (extLoginEl) {
				if (extHealthy && pairedClients > 0) {
					extLoginEl.textContent = '扩展: 已配对';
					extLoginEl.className = 'status-indicator connected';
				} else if (bridgeConnected && pairedClients === 0) {
					extLoginEl.textContent = '扩展: 桥接就绪';
					extLoginEl.className = 'status-indicator';
					extLoginEl.style.color = 'var(--warning)';
				} else {
					extLoginEl.textContent = '扩展: 未连接';
					extLoginEl.className = 'status-indicator disconnected';
				}
			}

			if (statusBadge) updateBridgeBadge(statusBadge, isConnected);
			// P2-6: 更新主页 mini 指示器
			const mini = document.getElementById('bridge-status-mini');
			if (mini) { mini.style.color = isConnected ? '#27ae60' : (bridgeOnline ? '#f39c12' : '#e74c3c'); mini.title = isConnected ? 'Bridge 已连接' : (bridgeOnline ? 'Bridge 部分可用' : 'Bridge 未连接'); }
			if (!statusInfo) {
				return;
			}

			if (effectiveMode === 'auto') {
				const parts = [];
				if (cdpHealthy) parts.push('CDP 在线');
				if (extHealthy) parts.push('扩展在线');
				if (parts.length === 0) {
					statusInfo.textContent = '自动模式：CDP 和扩展均不可用';
				} else {
					statusInfo.textContent = `自动模式：${parts.join('、')}（已配对 ${pairedClients}）`;
				}
				return;
			}
			if (effectiveMode === 'extension') {
				if (extHealthy) {
					statusInfo.textContent = `扩展在线（在线 ${liveClients} / 已配对 ${pairedClients}）`;
					return;
				}
				if (pairedClients > 0) {
					statusInfo.textContent = `扩展已配对但离线（已配对 ${pairedClients}）`;
					return;
				}
				if (bridgeConnected && pendingTasks > 0) {
					statusInfo.textContent = `扩展未配对（待处理任务 ${pendingTasks}）`;
					return;
				}
				if (bridgeConnected) {
					statusInfo.textContent = '桥接已就绪，请在扩展中完成配对';
					return;
				}
				statusInfo.textContent = '扩展桥接未连接，请检查 bridge 服务状态';
				return;
			}
			// cdp mode
			if (cdpHealthy) {
				statusInfo.textContent = 'CDP 浏览器已连接';
				return;
			}
			statusInfo.textContent = '当前截图引擎为 CDP，浏览器未连接';
		})
		.catch(err => {
			console.error('Bridge status error:', err);
			updateBridgeBadge(statusBadge, false);
			if (statusInfo) {
				statusInfo.textContent = '扩展状态检测失败';
			}
		});
}

function updateBridgeBadge(badge, connected) {
	if (!badge) {
		return;
	}
	badge.textContent = connected ? '扩展在线' : '扩展离线';
	badge.classList.toggle('cookie-status--on', connected);
	badge.classList.toggle('cookie-status--off', !connected);
}

function saveProxy(button, statusInfo) {
	const proxyServer = document.getElementById('proxy-server');
	if (!proxyServer) {
		alert('未找到代理输入框');
		return;
	}

	const formData = new FormData();
	formData.append('proxy_server', proxyServer.value || '');

	const originalText = button.textContent;
	button.textContent = '保存中...';
	button.disabled = true;

	apiFetch('/api/v1/cookies', {
		method: 'POST',
		body: formData
	})
		.then(parseJsonResponse)
		.then(data => {
			if (data && data.success) {
				const proxyValue = (proxyServer.value || '').trim();
				if (statusInfo) {
					statusInfo.textContent = proxyValue ? `代理已保存: ${proxyValue}` : '代理已清空并保存';
				}
				alert('代理设置已保存到配置文件');
			} else {
				alert('代理设置保存失败');
			}
		})
		.catch(err => {
			console.error('Save proxy error:', err);
			alert('代理设置保存失败');
		})
		.finally(() => {
			button.textContent = originalText;
			button.disabled = false;
		});
}

function refreshCDPStatus(statusBadge, statusInfo) {
	apiFetch('/api/v1/cdp/status')
		.then(parseJsonResponse)
		.then(data => {
			const online = data && data.online;
			const url = data && data.url ? data.url : '';
			if (statusBadge) updateCDPBadge(statusBadge, online);
			if (statusInfo) {
				if (online) {
					statusInfo.textContent = url ? `在线: ${url}` : '在线';
				} else if (data && data.error) {
					statusInfo.textContent = extractErrorMessage(data.error, '未连接');
				} else {
					statusInfo.textContent = '未连接';
				}
			}
			// P2-6: 更新主页 mini 指示器
			const mini = document.getElementById('cdp-status-mini');
			if (mini) { mini.style.color = online ? '#27ae60' : '#e74c3c'; mini.title = online ? 'CDP 已连接' : 'CDP 未连接'; }
		})
		.catch(err => {
			console.error('CDP status error:', err);
			if (statusBadge) updateCDPBadge(statusBadge, false);
			if (statusInfo) {
				statusInfo.textContent = '检测失败';
			}
			const mini = document.getElementById('cdp-status-mini');
			if (mini) { mini.style.color = '#e74c3c'; mini.title = 'CDP 检测失败'; }
		});
}

function connectCDP(button, statusBadge, statusInfo) {
	const proxyServer = document.getElementById('proxy-server');
	const originalText = button.textContent;
	button.textContent = '连接中...';
	button.disabled = true;

	const formData = new FormData();
	if (proxyServer) {
		formData.append('proxy_server', proxyServer.value || '');
	}

	apiFetch('/api/v1/cdp/connect', {
		method: 'POST',
		body: formData
	})
		.then(parseJsonResponse)
		.then(data => {
			const online = data && data.online;
			const url = data && data.url ? data.url : '';
			updateCDPBadge(statusBadge, online);
			if (statusInfo) {
				if (online) {
					statusInfo.textContent = url ? `在线: ${url}` : '在线';
				} else if (data && data.error) {
					const errMsg = extractErrorMessage(data.error, '连接失败');
					statusInfo.textContent = errMsg;
					handleBrowserError(errMsg);
				} else {
					statusInfo.textContent = '连接失败';
				}
			}
		})
		.catch(err => {
			console.error('CDP connect error:', err);
			updateCDPBadge(statusBadge, false);
			if (statusInfo) {
				statusInfo.textContent = '连接失败';
			}
		})
		.finally(() => {
			button.textContent = originalText;
			button.disabled = false;
		});
}

function updateCDPBadge(badge, online) {
	cdpOnline = !!online;
	if (!badge) return;
	badge.textContent = online ? '在线' : '未连接';
	badge.classList.toggle('cookie-status--on', online);
	badge.classList.toggle('cookie-status--off', !online);
}

// Returns whether browser query is enabled (reads from new radio group or legacy checkbox).
function isBrowserQueryModeEnabled() {
	const radios = document.querySelectorAll('input[name="browser-action"]');
	for (const radio of radios) {
		if (radio.checked) return true;
	}
	// Legacy fallback: if no radio group exists, check the old checkbox
	const checkbox = document.getElementById('browser-query-mode');
	return !!(checkbox && checkbox.checked);
}

// Returns the selected browser action: 'open', 'collect', 'collect_and_capture', or '' if none selected.
function getBrowserAction() {
	const radios = document.querySelectorAll('input[name="browser-action"]');
	for (const radio of radios) {
		if (radio.checked) return radio.value;
	}
	// Legacy fallback
	const checkbox = document.getElementById('browser-query-mode');
	if (checkbox && checkbox.checked) return 'collect';
	return '';
}

// Returns the selected screenshot mode: 'cdp', 'extension', or 'auto'.
function getSelectedScreenshotMode() {
	const radios = document.querySelectorAll('input[name="screenshot-mode"]');
	for (const radio of radios) {
		if (radio.checked) return radio.value;
	}
	return 'auto';
}

// Checks if the current screenshot mode has an available backend.
function isBrowserModeAvailable() {
	const mode = getSelectedScreenshotMode();
	if (mode === 'cdp') return cdpOnline;
	if (mode === 'extension') return bridgeOnline;
	return cdpOnline || bridgeOnline; // auto mode
}

function importCookieJSON(button) {
	const engine = document.getElementById('cookie-json-engine').value;
	const jsonText = document.getElementById('cookie-json').value;
	if (!jsonText.trim()) {
		alert('请粘贴 Cookie JSON');
		return;
	}

	const formData = new FormData();
	formData.append('engine', engine);
	formData.append('cookie_json', jsonText);

	const originalText = button.textContent;
	button.textContent = '导入中...';
	button.disabled = true;

	apiFetch('/api/v1/cookies/import', {
		method: 'POST',
		body: formData
	})
		.then(parseJsonResponse)
		.then(data => {
			if (data && data.success) {
				const inputId = `cookie-${engine}`;
				const input = document.getElementById(inputId);
				if (input && data.cookieHeader) {
					input.value = data.cookieHeader;
				}
				initCookieStatus();
				alert('Cookie JSON 导入成功');
			} else {
				alert(data && data.error ? extractErrorMessage(data.error, '导入失败') : '导入失败');
			}
		})
		.catch(err => {
			console.error('Import cookie json error:', err);
			alert('导入失败');
		})
		.finally(() => {
			button.textContent = originalText;
			button.disabled = false;
		});
}

function verifyCookies(button) {
	const query = document.getElementById('query').value;
	if (!query.trim()) {
		alert('请先输入查询语句');
		return;
	}

	const formData = new FormData();
	formData.append('query', query);

	const engineInputs = document.querySelectorAll('input[name="engines"]:checked');
	engineInputs.forEach(input => {
		formData.append('engines', input.value);
	});

	const fofa = document.getElementById('cookie-fofa');
	const hunter = document.getElementById('cookie-hunter');
	const zoomeye = document.getElementById('cookie-zoomeye');
	const quake = document.getElementById('cookie-quake');
	if (fofa && fofa.value) formData.append('cookie_fofa', fofa.value);
	if (hunter && hunter.value) formData.append('cookie_hunter', hunter.value);
	if (zoomeye && zoomeye.value) formData.append('cookie_zoomeye', zoomeye.value);
	if (quake && quake.value) formData.append('cookie_quake', quake.value);

	const resultBox = document.getElementById('cookie-verify-result');
	if (resultBox) {
		resultBox.textContent = '正在验证 Cookie，请稍候...';
	}

	const originalText = button.textContent;
	button.textContent = '验证中...';
	button.disabled = true;

	apiFetch('/api/v1/cookies/verify', {
		method: 'POST',
		body: formData
	})
		.then(parseJsonResponse)
		.then(data => {
			if (!resultBox) return;
			if (data && data.results) {
				resultBox.innerHTML = '';
				Object.keys(data.results).forEach(engine => {
					const item = data.results[engine];
					const ok = item && item.ok;
					const hint = item && item.hint ? String(item.hint) : '';
					const title = item && item.title ? String(item.title) : '';

					const row = document.createElement('div');
					const status = document.createElement('span');
					status.className = ok ? 'ok' : 'fail';
					status.textContent = ok ? '正常' : '异常';

					row.appendChild(document.createTextNode(`${String(engine)}: `));
					row.appendChild(status);
					if (title) {
						row.appendChild(document.createTextNode(` - ${title}`));
					}
					if (hint) {
						row.appendChild(document.createTextNode(` (${hint})`));
					}

					resultBox.appendChild(row);
				});
			} else if (data && data.error) {
				resultBox.textContent = extractErrorMessage(data.error, '验证失败');
			} else {
				resultBox.textContent = '验证失败，请稍后重试。';
			}
		})
		.catch(err => {
			console.error('Verify cookies error:', err);
			if (resultBox) {
				resultBox.textContent = '验证失败，请稍后重试。';
			}
		})
		.finally(() => {
			button.textContent = originalText;
			button.disabled = false;
		});
}

function initCookieStatus() {
	const map = [
		{ input: 'cookie-fofa', badge: 'cookie-status-fofa' },
		{ input: 'cookie-hunter', badge: 'cookie-status-hunter' },
		{ input: 'cookie-zoomeye', badge: 'cookie-status-zoomeye' },
		{ input: 'cookie-quake', badge: 'cookie-status-quake' }
	];

	map.forEach(item => {
		const input = document.getElementById(item.input);
		const badge = document.getElementById(item.badge);
		if (!input || !badge) return;
		input.addEventListener('input', function() {
			updateCookieBadge(badge, input.value);
		});
		updateCookieBadge(badge, input.value);
	});
}

function updateCookieBadge(badge, value) {
	const hasValue = value && value.trim().length > 0;
	badge.textContent = hasValue ? '已配置' : '未配置';
	badge.classList.toggle('cookie-status--on', hasValue);
	badge.classList.toggle('cookie-status--off', !hasValue);
}

function clearCookies(button) {
	if (!confirm('确定要清空所有引擎 Cookie 吗？')) {
		return;
	}

	const fofa = document.getElementById('cookie-fofa');
	const hunter = document.getElementById('cookie-hunter');
	const zoomeye = document.getElementById('cookie-zoomeye');
	const quake = document.getElementById('cookie-quake');
	if (fofa) fofa.value = '';
	if (hunter) hunter.value = '';
	if (zoomeye) zoomeye.value = '';
	if (quake) quake.value = '';

	const formData = new FormData();
	formData.append('clear_cookies', 'true');

	const originalText = button.textContent;
	button.textContent = '清空中...';
	button.disabled = true;

	apiFetch('/api/v1/cookies', {
		method: 'POST',
		body: formData
	})
		.then(parseJsonResponse)
		.then(data => {
			if (data && data.success) {
				initCookieStatus();
				alert('Cookie 已清空');
			} else {
				alert('Cookie 清空失败');
			}
		})
		.catch(err => {
			console.error('Clear cookies error:', err);
			alert('Cookie 清空失败');
		})
		.finally(() => {
			button.textContent = originalText;
			button.disabled = false;
		});
}

function saveCookies(button) {
	const fofa = document.getElementById('cookie-fofa');
	const hunter = document.getElementById('cookie-hunter');
	const zoomeye = document.getElementById('cookie-zoomeye');
	const quake = document.getElementById('cookie-quake');
	const proxyServer = document.getElementById('proxy-server');

	const formData = new FormData();
	if (fofa && fofa.value) {
		formData.append('cookie_fofa', fofa.value);
	}
	if (hunter && hunter.value) {
		formData.append('cookie_hunter', hunter.value);
	}
	if (zoomeye && zoomeye.value) {
		formData.append('cookie_zoomeye', zoomeye.value);
	}
	if (quake && quake.value) {
		formData.append('cookie_quake', quake.value);
	}
	if (proxyServer) {
		formData.append('proxy_server', proxyServer.value || '');
	}

	const hasCookies = !!((fofa && fofa.value) || (hunter && hunter.value) || (zoomeye && zoomeye.value) || (quake && quake.value));
	const hasProxy = !!(proxyServer && proxyServer.value && proxyServer.value.trim());
	if (!hasCookies && !hasProxy) {
		alert('请先填写至少一个 Cookie 或代理地址');
		return;
	}

	const originalText = button.textContent;
	button.textContent = '保存中...';
	button.disabled = true;

	apiFetch('/api/v1/cookies', {
		method: 'POST',
		body: formData
	})
		.then(parseJsonResponse)
		.then(data => {
			if (data && data.success) {
				initCookieStatus();
				alert('Cookie/代理设置已保存到配置文件');
			} else {
				alert('Cookie/代理设置保存失败');
			}
		})
		.catch(err => {
			console.error('Save cookies error:', err);
			alert('Cookie/代理设置保存失败');
		})
		.finally(() => {
			button.textContent = originalText;
			button.disabled = false;
		});
}

// WebSocket连接管理
let wsConnection = null;
let wsConnected = false;
let wsPingTimer = null;
let wsReconnectAttempts = 0;
let wsReconnectTimer = null;
let currentQueryID = null;
let cdpOnline = false;
let bridgeOnline = false; // tracks extension live_clients > 0

// CDP/Bridge 状态轮询 interval ID（initCDPControls/initBridgeStatusControls 写入）
let cdpStatusTimer = null;
let bridgeStatusTimer = null;
let engineStatusMap = {}; // { engineName: { hasKey: bool, enabled: bool } }
let currentQueryTimeout = null; // P1-2: 查询超时计时器
const QUERY_CLIENT_TIMEOUT_MS = 5 * 60 * 1000;
const QUERY_CLIENT_TIMEOUT_LABEL = '5 分钟';
let currentAssets = []; // P2-2: 当前查询结果数据（用于分页渲染）
let filteredAssets = null; // P2-2: 筛选后的子集（null 表示未筛选）

// ============================================================
// 登录状态检测
// ============================================================

let loginStatusPollInterval = null;

// 清理所有轮询定时器和 WebSocket 连接。
// 在 beforeunload 事件中调用，防止页面离开后定时器继续执行。
function stopAllPolling() {
	if (cdpStatusTimer) { clearInterval(cdpStatusTimer); cdpStatusTimer = null; }
	if (bridgeStatusTimer) { clearInterval(bridgeStatusTimer); bridgeStatusTimer = null; }
	if (loginStatusPollInterval) { clearInterval(loginStatusPollInterval); loginStatusPollInterval = null; }
	if (wsPingTimer) { clearInterval(wsPingTimer); wsPingTimer = null; }
	if (wsReconnectTimer) { clearTimeout(wsReconnectTimer); wsReconnectTimer = null; }
	if (wsConnection) {
		// 关闭前移除 onclose 回调，避免触发新一轮重连
		wsConnection.onclose = null;
		wsConnection.close();
		wsConnection = null;
	}
}

function refreshLoginStatus() {
	const query = document.getElementById('query');
	const queryVal = query && query.value.trim() ? query.value.trim() : 'protocol="http"';

	apiFetch('/api/v1/cookies/login-status?query=' + encodeURIComponent(queryVal))
		.then(parseJsonResponse)
		.then(data => {
			if (!data || !data.success) return;
			updateCDPStatus(data.cdp_connected);
			updateExtStatus(data.ext_paired);
			if (data.engines && Array.isArray(data.engines)) {
				updateEngineLoginStatus(data.engines);
			}
		})
		.catch(err => {
			console.error('Failed to fetch login status:', err);
		});
}

function updateCDPStatus(connected) {
	cdpOnline = !!connected;
	const el = document.getElementById('status-cdp');
	if (!el) return;
	if (connected) {
		el.textContent = 'CDP: 已连接';
		el.className = 'status-indicator connected';
	} else {
		el.textContent = 'CDP: 未连接';
		el.className = 'status-indicator disconnected';
	}
}

function updateExtStatus(paired) {
	bridgeOnline = !!paired;
	const el = document.getElementById('status-ext');
	if (!el) return;
	if (paired) {
		el.textContent = '扩展: 已配对';
		el.className = 'status-indicator connected';
	} else {
		el.textContent = '扩展: 未配对';
		el.className = 'status-indicator disconnected';
	}
}

function updateEngineLoginStatus(engines) {
	let anyNotLoggedIn = false;

	engines.forEach(function(engine) {
		var statusEl = document.getElementById('engine-status-' + engine.engine);
		var loginBtn = document.getElementById('btn-login-' + engine.engine);
		if (!statusEl) return;

		if (engine.logged_in) {
			statusEl.textContent = '\u2713 已登录';
			statusEl.className = 'engine-status-text logged-in';
			if (loginBtn) loginBtn.style.display = 'none';
		} else {
			anyNotLoggedIn = true;
			switch (engine.reason) {
				case 'cookie_configured':
					statusEl.textContent = 'Cookie 已配置（headless 模式）';
					statusEl.className = 'engine-status-text logged-in';
					break;
				case 'login_required':
					statusEl.textContent = '需要登录';
					statusEl.className = 'engine-status-text not-logged-in';
					if (loginBtn) {
						loginBtn.href = engine.login_url || '#';
						loginBtn.style.display = 'inline-block';
					}
					break;
				case 'no_session':
					statusEl.textContent = '无浏览器会话';
					statusEl.className = 'engine-status-text no-session';
					if (loginBtn) {
						loginBtn.href = engine.login_url || '#';
						loginBtn.style.display = 'inline-block';
					}
					break;
				case 'browser_session':
					statusEl.textContent = '✓ 已登录';
					statusEl.className = 'engine-status-text logged-in';
					break;
				case 'page_too_short':
					statusEl.textContent = '页面加载异常';
					statusEl.className = 'engine-status-text not-logged-in';
					if (loginBtn) {
						loginBtn.href = engine.login_url || '#';
						loginBtn.style.display = 'inline-block';
					}
					break;
				case 'extension_paired_session_unverified':
					statusEl.textContent = '扩展已配对（登录未验证）';
					statusEl.className = 'engine-status-text not-logged-in';
					break;
				default:
					statusEl.textContent = engine.error || '状态未知';
					statusEl.className = 'engine-status-text no-session';
			}
		}
	});

	// 自动折叠/展开逻辑
	var content = document.getElementById('cookie-input-content');
	var arrow = document.querySelector('.toggle-arrow');
	if (!anyNotLoggedIn && content && arrow) {
		content.style.display = 'none';
		arrow.classList.remove('expanded');
	} else if (anyNotLoggedIn && content && arrow) {
		content.style.display = 'block';
		arrow.classList.add('expanded');
	}
}

function toggleCookieSection() {
	var content = document.getElementById('cookie-input-content');
	var arrow = document.querySelector('.toggle-arrow');
	if (!content || !arrow) return;
	var isVisible = content.style.display === 'block';
	content.style.display = isVisible ? 'none' : 'block';
	arrow.classList.toggle('expanded', !isVisible);
}

// 初始化登录状态轮询
function initLoginStatusPoll() {
	// 不立即刷新，等待 15s 后首次执行，避免页面打开时自动执行查询检查
	if (loginStatusPollInterval) {
		clearInterval(loginStatusPollInterval);
	}
	loginStatusPollInterval = setInterval(refreshLoginStatus, 15000);
}

// 初始化WebSocket连接
function initWebSocket() {
	// 关闭现有连接
	if (wsConnection) {
		wsConnection.close();
	}

	// 创建新连接
	const wsProtocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
	const wsUrl = wsProtocol + window.location.host + '/api/v1/ws';
	wsConnection = new WebSocket(wsUrl);

	wsConnection.onopen = function() {
		console.log('WebSocket connected');
		wsConnected = true;
		wsReconnectAttempts = 0;
		// 发送ping消息保持连接
		startPingInterval();
	};

	wsConnection.onmessage = function(event) {
		try {
			const message = JSON.parse(event.data);
			handleWebSocketMessage(message);
		} catch (err) {
			console.error('WebSocket message parse error:', err);
		}
	};

	wsConnection.onclose = function() {
		console.log('WebSocket disconnected');
		wsConnected = false;
		if (wsPingTimer) { clearInterval(wsPingTimer); wsPingTimer = null; }

		// 指数退避重连：5s → 10s → 20s → 40s → 60s(上限)，最多 6 次
		if (wsReconnectTimer) { clearTimeout(wsReconnectTimer); wsReconnectTimer = null; }
		const maxAttempts = 6;
		if (wsReconnectAttempts >= maxAttempts) {
			console.warn('WebSocket reconnection limit reached, giving up');
			// P2-14: 显示重连失败提示
			showWSDisconnectedBanner();
			return;
		}
		const delay = Math.min(5000 * Math.pow(2, wsReconnectAttempts), 60000);
		console.log('WebSocket will reconnect in ' + (delay / 1000) + 's (attempt ' + (wsReconnectAttempts + 1) + '/' + maxAttempts + ')');
		wsReconnectAttempts++;
		wsReconnectTimer = setTimeout(initWebSocket, delay);
	};

	wsConnection.onerror = function(error) {
		console.error('WebSocket error:', error);
	};
}

// P2-14: WebSocket 断连提示 banner
function showWSDisconnectedBanner() {
	let banner = document.getElementById('ws-disconnected-banner');
	if (!banner) {
		banner = document.createElement('div');
		banner.id = 'ws-disconnected-banner';
		banner.style.cssText = 'background:#f8d7da; border:1px solid #f5c6cb; color:#721c24; padding:10px 16px; border-radius:6px; margin-bottom:12px; font-size:14px; display:flex; justify-content:space-between; align-items:center;';
		const span = document.createElement('span');
		span.textContent = '⚠️ 实时连接已断开，查询进度不可用。';
		const btn = document.createElement('button');
		btn.className = 'btn btn-sm btn-primary';
		btn.textContent = '重新连接';
		btn.addEventListener('click', function() {
			banner.remove();
			initWebSocket();
		});
		banner.appendChild(span);
		banner.appendChild(btn);
		const main = document.querySelector('main');
		if (main && main.firstChild) {
			main.insertBefore(banner, main.firstChild);
		}
	}
	banner.style.display = 'flex';
}

// P2-15: 网络离线检测
window.addEventListener('offline', function() {
	let banner = document.getElementById('offline-banner');
	if (!banner) {
		banner = document.createElement('div');
		banner.id = 'offline-banner';
		banner.style.cssText = 'background:#fff3cd; border:1px solid #ffc107; color:#856404; padding:10px 16px; border-radius:6px; margin-bottom:12px; font-size:14px; text-align:center;';
		banner.textContent = '📡 网络已断开，请检查网络连接。';
		const main = document.querySelector('main');
		if (main && main.firstChild) {
			main.insertBefore(banner, main.firstChild);
		}
	}
	banner.style.display = 'block';
});
window.addEventListener('online', function() {
	const banner = document.getElementById('offline-banner');
	if (banner) banner.style.display = 'none';
});

// 发送ping消息保持连接
function startPingInterval() {
	if (wsPingTimer) clearInterval(wsPingTimer);
	wsPingTimer = setInterval(() => {
		if (wsConnected && wsConnection && wsConnection.readyState === WebSocket.OPEN) {
			wsConnection.send(JSON.stringify({ type: 'ping' }));
		}
	}, 30000);
}

// P2-16: 统一 fetch 包装器 — 自动处理 401/403/429
function apiFetch(url, options) {
	return fetch(url, options).then(function(resp) {
		if (resp.status === 401) {
			// session 过期，跳转登录
			window.location.href = '/login';
			throw new Error('会话已过期，请重新登录');
		}
		if (resp.status === 403) {
			showMessage('权限不足，无法执行此操作', 'error');
			throw new Error('权限不足');
		}
		if (resp.status === 429) {
			var retry = resp.headers.get('Retry-After');
			var msg = '请求过于频繁，请稍后再试';
			if (retry) msg += '（' + retry + ' 秒后可重试）';
			showMessage(msg, 'error');
			throw new Error('rate_limited');
		}
		return resp;
	});
}

// 解析 JSON 响应，统一检查 resp.ok
function parseJsonResponse(resp) {
	if (!resp.ok) throw new Error('请求失败 (' + resp.status + ')');
	return resp.json();
}

// 处理WebSocket消息
function handleWebSocketMessage(message) {
	switch (message.type) {
		case 'pong':
			// 心跳响应，无需处理
			break;
		case 'query_start':
			handleQueryStart(message);
			break;
		case 'progress_update':
			handleProgressUpdate(message);
			break;
		case 'query_complete':
			handleQueryComplete(message);
			break;
		case 'query_error':
			handleQueryError(message);
			break;
	}
}

// 处理查询错误
function handleQueryError(message) {
	// P1-2: 清除超时计时器
	if (currentQueryTimeout) { clearTimeout(currentQueryTimeout); currentQueryTimeout = null; }

	// 移除 loading 指示器
	removeLoadingIndicator();

	// 恢复按钮状态
	const submitBtn = document.querySelector('button[type="submit"]');
	if (submitBtn) {
		submitBtn.textContent = '执行查询';
		submitBtn.disabled = false;
		submitBtn.classList.remove('loading');
	}

	showResultsError(extractErrorMessage(message.error, '查询失败'));
}

// 提取错误消息文本（兼容 string 和 {code, message} 对象格式）
function extractErrorMessage(error, fallback) {
	if (!error) return fallback || '未知错误';
	if (typeof error === 'string') return error;
	if (typeof error === 'object' && error.message) return error.message;
	return fallback || JSON.stringify(error);
}

function escapeHtml(value) {
	const str = value === null || value === undefined ? '' : String(value);
	return str
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

function escapeAttr(value) {
	return escapeHtml(value).replace(/`/g, '&#96;');
}

function sanitizePreviewPath(path) {
	if (!path) return '';
	const str = String(path).trim();
	if (!str.startsWith('/screenshots/')) return '';
	if (str.includes('..')) return '';
	return str;
}

// 对引擎错误消息进行友好化分类，返回 {html, category}。
// 仅做前端映射，不修改后端 adapter 错误字符串。
function classifyEngineError(msg) {
	if (!msg) return { html: '<span class="err-cat err-unknown">未知</span> 未知错误', category: 'unknown' };
	var s = String(msg).toLowerCase();
	var cat, friendly;

	// 额度/余额不足
	if (s.indexOf('402') !== -1 || s.indexOf('payment') !== -1 || s.indexOf('余额不足') !== -1
		|| s.indexOf('f点') !== -1 || s.indexOf('credits_insufficient') !== -1
		|| s.indexOf('insufficient') !== -1 || s.indexOf('quota') !== -1) {
		cat = '额度';
		friendly = '额度/余额不足，请充值或更换 API Key';
	}
	// 速率限制
	else if (s.indexOf('429') !== -1 || s.indexOf('rate limit') !== -1
		|| s.indexOf('请求太多') !== -1 || s.indexOf('too many') !== -1
		|| s.indexOf('频率') !== -1) {
		cat = '限流';
		friendly = '请求频率超限，请稍后重试';
	}
	// 权限不足
	else if (s.indexOf('403') !== -1 || s.indexOf('membership') !== -1
		|| s.indexOf('requires') !== -1 || s.indexOf('付费') !== -1) {
		cat = '权限';
		friendly = '需要更高权限或付费会员';
	}
	// 认证失败
	else if (s.indexOf('401') !== -1 || s.indexOf('unauthorized') !== -1
		|| s.indexOf('authentication') !== -1 || s.indexOf('api key') !== -1) {
		cat = '认证';
		friendly = '认证失败，请 <a href="/settings#panel-engines">配置 API Key</a>';
	}
	else {
		cat = '错误';
		friendly = escapeHtml(msg);
		return { html: '<span class="err-cat err-other">' + escapeHtml(cat) + '</span> ' + friendly, category: 'other' };
	}

	return {
		html: '<span class="err-cat err-' + escapeHtml(cat) + '">' + escapeHtml(cat) + '</span> ' + escapeHtml(friendly),
		category: cat
	};
}

// 处理查询开始
// 移除查询结果页的 loading 指示器
function removeLoadingIndicator() {
	const loading = document.querySelector('.loading-indicator');
	if (loading) loading.remove();
}

function handleQueryStart(message) {
	currentQueryID = message.query_id;
	const status = message.status;
	const queryID = escapeHtml(status && status.ID ? status.ID : '');
	const queryStatus = escapeHtml(status && status.Status ? status.Status : '');
	const startTime = status && status.StartTime ? new Date(status.StartTime).toLocaleString() : '';

	// 更新结果页面
	const resultsContent = document.getElementById('results-content');
	if (resultsContent) {
		resultsContent.innerHTML = `
			<div class="query-status">
				<h3>查询状态</h3>
				<p>查询ID: ${queryID}</p>
				<p>状态: ${queryStatus}</p>
				<p>进度: <span id="progress-bar">0%</span></p>
				<div class="progress-container">
					<div id="progress-fill" class="progress-fill" style="width: 0%"></div>
				</div>
				<p>开始时间: ${escapeHtml(startTime)}</p>
			</div>
		`;
	}
}

// 处理进度更新
function handleProgressUpdate(message) {
	const progress = message.progress;
	const progressBar = document.getElementById('progress-bar');
	const progressFill = document.getElementById('progress-fill');

	if (progressBar) {
		progressBar.textContent = `${progress.toFixed(1)}%`;
	}

	if (progressFill) {
		progressFill.style.width = `${progress}%`;
	}
}

// 处理查询完成
function handleQueryComplete(message) {
	// P1-2: 清除超时计时器
	if (currentQueryTimeout) { clearTimeout(currentQueryTimeout); currentQueryTimeout = null; }

	// 移除 loading 指示器
	removeLoadingIndicator();

	const results = message.results;

	// 恢复按钮状态
	const submitBtn = document.querySelector('button[type="submit"]');
	if (submitBtn) {
		submitBtn.textContent = '执行查询';
		submitBtn.disabled = false;
		submitBtn.classList.remove('loading');
	}

	// 显示结果
	if (results.error) {
		showResultsError(extractErrorMessage(results.error));
	} else {
		showResults(results);
	}
}

// 执行异步查询（WebSocket版本）
function executeAsyncQuery(query, engines, apiEngines, submitBtn, originalText, browserQuery, browserAction) {
	const safeQuery = escapeHtml(query);
	const safeEnginesText = engines.map(engine => escapeHtml(engine)).join(', ');

	// 创建结果页面
	const resultsPage = document.createElement('div');
	resultsPage.className = 'results-page';
	resultsPage.innerHTML = `
		<div class="results-header">
			<h2>查询结果</h2>
			<p>查询语句: <code>${safeQuery}</code></p>
			<p>使用引擎: ${safeEnginesText}</p>
			<p>浏览器查询: ${browserQuery ? '已开启' : '未开启'}${browserAction ? ' (' + browserAction + ')' : ''}</p>
			<div class="loading-indicator">
				<div class="spinner"></div>
				<p>正在查询...请稍候</p>
			</div>
		</div>
		<div id="results-content" class="results-content">
			<!-- 结果将在这里动态加载 -->
		</div>
	`;
	
	// 替换当前页面内容
	const main = document.querySelector('main');
	main.innerHTML = '';
	main.appendChild(resultsPage);

	// 检查WebSocket连接
	if (!wsConnected || wsConnection.readyState !== WebSocket.OPEN) {
		// WebSocket未连接，使用传统API
		useFallbackAPI(query, browserQuery ? apiEngines : engines, submitBtn, originalText, browserQuery, browserAction);
		return;
	}

	// 使用WebSocket执行查询
	wsConnection.send(JSON.stringify({
		type: 'query',
		query: query,
		engines: engines,
		api_engines: browserQuery ? apiEngines : engines,
		page_size: 50,
		browser_query: !!browserQuery,
		browser_action: browserAction || '',
	}));

	// P1-2: 客户端超时检测
	currentQueryTimeout = setTimeout(function() {
		removeLoadingIndicator();
		const resultsContent = document.getElementById('results-content');
		if (resultsContent) {
			const wrap = document.createElement('div');
			wrap.style.cssText = 'color:#856404; background:#fff3cd; padding:16px; border-radius:6px; border:1px solid #ffc107;';
			const h4 = document.createElement('h4');
			h4.style.margin = '0 0 8px';
			h4.textContent = '查询超时';
			const p = document.createElement('p');
			p.style.margin = '0';
			p.textContent = '查询已超过 ' + QUERY_CLIENT_TIMEOUT_LABEL + ' 未响应，可能是引擎响应缓慢或网络问题。';
			const btn = document.createElement('button');
			btn.className = 'btn btn-primary';
			btn.style.marginTop = '12px';
			btn.textContent = '重新查询';
			btn.addEventListener('click', function() { location.reload(); });
			wrap.appendChild(h4);
			wrap.appendChild(p);
			wrap.appendChild(btn);
			resultsContent.innerHTML = '';
			resultsContent.appendChild(wrap);
		}
		// 恢复按钮状态
		if (submitBtn) {
			submitBtn.textContent = originalText;
			submitBtn.disabled = false;
			submitBtn.classList.remove('loading');
		}
	}, QUERY_CLIENT_TIMEOUT_MS);
}

// 传统API回退方案
function useFallbackAPI(query, engines, submitBtn, originalText, browserQuery, browserAction) {
	// P1-2: AbortController 超时控制
	const controller = new AbortController();
	const timeoutId = setTimeout(function() { controller.abort(); }, QUERY_CLIENT_TIMEOUT_MS);

	// 发送API请求
	apiFetch('/api/v1/query', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
		},
		body: new URLSearchParams({
			'query': query,
			'engines': engines.join(','),
			'page_size': '50',
			'browser_query': browserQuery ? 'true' : 'false',
			'browser_action': browserAction || '',
		}),
		signal: controller.signal,
	})
	.then(parseJsonResponse)
	.then(data => {
		clearTimeout(timeoutId);
		removeLoadingIndicator();
		// 恢复按钮状态
		if (submitBtn) {
			submitBtn.textContent = originalText;
			submitBtn.disabled = false;
			submitBtn.classList.remove('loading');
		}

		// 显示结果
		if (data.error) {
			const errMsg = extractErrorMessage(data.error);
			handleBrowserError(errMsg);
			showResultsError(errMsg);
		} else {
			showResults(data);
		}
	})
	.catch(error => {
		clearTimeout(timeoutId);
		removeLoadingIndicator();
		// 恢复按钮状态
		if (submitBtn) {
			submitBtn.textContent = originalText;
			submitBtn.disabled = false;
			submitBtn.classList.remove('loading');
		}

		// 显示错误
		if (error.name === 'AbortError') {
			showResultsError('查询超时（超过 ' + QUERY_CLIENT_TIMEOUT_LABEL + '），请稍后重试。');
		} else {
			showResultsError('查询失败: ' + error.message);
		}
	});
}

// 查询结果操作委托（CSP 兼容：替代 innerHTML 中的 onclick 属性）
function initResultsActionDelegation() {
	var resultsContent = document.getElementById('results-content');
	if (!resultsContent || resultsContent.dataset.actionDelegated === '1') return;
	resultsContent.dataset.actionDelegated = '1';

	resultsContent.addEventListener('click', function(e) {
		var btn = e.target.closest('[data-action]');
		if (!btn) return;
		var action = btn.getAttribute('data-action');
		switch (action) {
			case 'go-home':
				window.location.href = '/';
				break;
			case 'capture-all-screenshots':
				captureAllScreenshots();
				break;
			case 'capture-search-engine-screenshots':
				captureSearchEngineScreenshots();
				break;
		}
	});
}

// 错误信息折叠展开：直接绑定到每个 .errors-header。每次 innerHTML 渲染
// 产生的都是全新元素，旧绑定随旧元素一起销毁，故无重复绑定风险。
// 不依赖容器级委托——渲染流程中若其他初始化（initResultsTable 等）抛
// 异常会中断委托绑定，导致点击无反应；直接绑定放在 innerHTML 之后、
// 其他 init 之前，确保不受影响。
function bindErrorToggles() {
	var headers = document.querySelectorAll('.errors-header');
	for (var i = 0; i < headers.length; i++) {
		(function(header) {
			header.addEventListener('click', function() {
				var box = header.closest('.errors-collapsible');
				if (!box) return;
				box.classList.toggle('expanded');
				var arrow = header.querySelector('.toggle-arrow');
				if (arrow) arrow.classList.toggle('expanded');
			});
		})(headers[i]);
	}
}

// 显示查询错误
function showResultsError(error) {
	var resultsContent = document.getElementById('results-content');
	if (resultsContent) {
		var safeError = escapeHtml(error);
		resultsContent.innerHTML =
			'<div class="error-message">' +
				'<h3>查询错误</h3>' +
				'<p>' + safeError + '</p>' +
				'<button type="button" class="btn btn-primary" data-action="go-home">返回首页</button>' +
			'</div>';
	}
}

// 显示查询结果
function showResults(data) {
	const resultsContent = document.getElementById('results-content');
	if (resultsContent) {
		// Normalize field names (WS vs HTTP) and asset shapes
		const assets = (data && (data.assets || data.Assets)) || [];
		currentAssets = assets; // P2-2: 存储全局数据用于分页渲染
		const totalCount = (data && (data.totalCount ?? data.TotalCount)) ?? (Array.isArray(assets) ? assets.length : 0);
		const engineStats = data && (data.engineStats || data.EngineStats);
		const errors = (data && (data.errors || data.Errors)) || [];
		const browserQuery = !!(data && (data.browserQuery ?? data.browser_query));
		const browserOpenedEngines = (data && (data.browserOpenedEngines || data.browser_opened_engines)) || [];
		const browserQueryErrors = (data && (data.browserQueryErrors || data.browser_query_errors)) || [];
		const autoCapture = !!(data && (data.autoCapture ?? data.auto_capture));
		const autoCaptureQueryID = (data && (data.autoCaptureQueryID || data.auto_capture_query_id)) || '';
		const autoCapturedPaths = (data && (data.autoCapturedPaths || data.auto_captured_paths)) || {};
		const autoCaptureErrors = (data && (data.autoCaptureErrors || data.auto_capture_errors)) || [];

		function pick(obj, ...keys) {
			if (!obj) return '';
			for (const key of keys) {
				if (obj[key] !== undefined && obj[key] !== null) return obj[key];
			}
			return '';
		}

		// 构建结果HTML
		let html = `
			<div class="results-info">
				<p>总结果数: ${totalCount}</p>
			</div>
		`;

		if (browserQuery) {
			const openedText = browserOpenedEngines.length > 0 ? browserOpenedEngines.map(engine => escapeHtml(engine)).join(', ') : '无';
			html += `
				<div class="results-info">
					<p>浏览器查询模式: 已开启</p>
					<p>已打开引擎: ${openedText}</p>
				</div>
			`;
		}

		if (autoCapture) {
			const capturedEntries = Object.entries(autoCapturedPaths);
			const capturedEngineText = capturedEntries.length > 0 ? capturedEntries.map(([engine]) => escapeHtml(engine)).join(', ') : '无';
			html += `
				<div class="results-info">
					<p>自动截图: 已开启</p>
					<p>截图批次ID: ${escapeHtml(autoCaptureQueryID || '未生成')}</p>
					<p>已截图引擎: ${capturedEngineText}</p>
				</div>
			`;

			if (capturedEntries.length > 0) {
				html += `
					<div class="results-info">
						<h3>自动截图路径</h3>
						<ul>
							${capturedEntries.map(([engine, path]) => {
								const safeEngine = escapeHtml(engine);
								const previewPath = sanitizePreviewPath(path);
								if (!previewPath) {
									return `<li>${safeEngine}: ${escapeHtml('不可预览（路径无效）')}</li>`;
								}
								const safeHref = escapeAttr(previewPath);
								const safeText = escapeHtml(previewPath);
								return `<li>${safeEngine}: <a href="${safeHref}" target="_blank" rel="noopener noreferrer">${safeText}</a></li>`;
							}).join('')}
						</ul>
					</div>
				`;
			}
		}
		
		// 显示错误信息（折叠+友好化）
		const combinedErrors = Array.from(new Set(errors.concat(browserQueryErrors).concat(autoCaptureErrors)));
		if (combinedErrors && combinedErrors.length > 0) {
			const hasAssets = assets && assets.length > 0;
			const titlePrefix = hasAssets ? '⚠ 部分引擎未返回结果' : '⚠ 查询失败';
			const titleText = `${titlePrefix}（${combinedErrors.length} 个，点击展开）`;
			html += `
				<div class="errors errors-collapsible">
					<div class="errors-header">
						<span class="toggle-arrow">▶</span>
						<span>${escapeHtml(titleText)}</span>
					</div>
					<ul class="errors-body">
						${combinedErrors.map(err => {
							const friendly = classifyEngineError(err);
							return '<li>' + friendly.html + '</li>';
						}).join('')}
					</ul>
				</div>
			`;
		}
		
		// 显示引擎统计
		if (engineStats) {
			html += `
				<div class="engine-stats">
					<h3>引擎统计</h3>
					<div class="stats-grid">
						${Object.entries(engineStats).map(([engine, count]) => `
							<div class="stat-item">
								<span class="engine-name">${escapeHtml(engine)}</span>
								<span class="count">${escapeHtml(count)}</span>
							</div>
						`).join('')}
					</div>
				</div>
			`;
		}
		
		// 显示结果表格
		if (Array.isArray(assets) && assets.length > 0) {
			html += `
				<div class="filter-bar">
					<strong>结果筛选:</strong>
					<input type="text" id="filter-ip" placeholder="IP" class="form-control">
					<input type="text" id="filter-port" placeholder="端口" class="form-control">
					<input type="text" id="filter-protocol" placeholder="协议" class="form-control">
					<input type="text" id="filter-source" placeholder="来源" class="form-control">
					<button id="btn-apply-filter" class="btn btn-sm btn-secondary">筛选</button>
					<button id="btn-reset-filter" class="btn btn-sm btn-secondary">重置</button>
					<span class="filter-count">
						显示: <span id="displayed-count">${assets.length}</span> / <span id="total-count">${assets.length}</span>
					</span>
				</div>

				<div class="results-table-container">
					<table class="results-table">
						<thead>
							<tr>
								<th>IP</th>
								<th>端口</th>
								<th>协议</th>
								<th>主机</th>
								<th>标题</th>
								<th>服务器</th>
								<th>状态码</th>
								<th>来源</th>
								<th>操作</th>
							</tr>
						</thead>
						<tbody id="results-body">
							<!-- P2-2: 首页只渲染前50行，其余由分页动态渲染 -->
						</tbody>
					</table>
				</div>
			`;
		} else {
			html += `
				<div class="no-results">
					<h3>未找到结果</h3>
					<p>当前查询条件下没有找到任何资产。</p>
				</div>
			`;
		}

		// 添加截图操作栏
		html += `
			<div class="screenshot-actions">
				<h4>📸 截图功能</h4>
				<div class="screenshot-btns">
					<button type="button" id="btn-screenshot-all" class="btn btn-primary" data-action="capture-all-screenshots">
						批量截图所有结果
					</button>
					<button type="button" id="btn-screenshot-search-engines" class="btn btn-info" data-action="capture-search-engine-screenshots">
						截图搜索引擎结果页
					</button>
					<span id="screenshot-status" class="screenshot-status"></span>
				</div>
				<div id="screenshot-progress" class="hidden">
					<div class="progress-container">
						<div id="screenshot-progress-bar" class="progress-bar"></div>
					</div>
					<p id="screenshot-progress-text" class="progress-text"></p>
				</div>
			</div>
		`;

		// 添加返回按钮
		html += `
			<div class="results-footer">
				<button type="button" class="btn btn-secondary" data-action="go-home">返回首页</button>
			</div>
		`;
		
		// 更新结果内容
		resultsContent.innerHTML = html;

		// 直接绑定错误折叠展开（须在其他 init 之前，避免后续异常中断绑定）
		bindErrorToggles();

		// 保存当前查询数据供截图使用
		window.currentQueryData = {
			query: data.query || '',
			engines: data.engines || [],
			assets: assets,
			queryID: 'query_' + Date.now(),
			browserQuery: browserQuery
		};
		
		// 保存到服务端历史
		try { saveQueryToServerHistory(data.query || '', data.engines || [], data); } catch(e) { console.error('saveQueryToServerHistory:', e); }

		// 用 try/finally 保证：即使某个 init 抛异常，结果行也一定渲染，
		// 错误折叠展开也一定绑定。否则任一 init 异常会中断后续所有绑定，
		// 导致表格行/错误展开都不工作。
		try {
			// 初始化结果表格功能
			initResultsTable();
			// 事件委托：资产详情/复制/截图按钮（避免翻页重复绑定）
			initAssetActionDelegation();
			// CSP兼容：注册结果操作事件委托
			initResultsActionDelegation();
		} catch(e) { console.error('results init:', e); }
		try {
			// P2-2: 渲染第一页数据
			renderAssetRows(0, 50);
		} catch(e) { console.error('renderAssetRows:', e); }
	}
}

// P2-2: 渲染指定范围的资产行（从 currentAssets）
function renderAssetRows(start, end) {
	renderAssetRowsFrom(currentAssets, start, end);
}

// P2-2: 从指定数组渲染指定范围的资产行
function renderAssetRowsFrom(assets, start, end) {
	const tbody = document.getElementById('results-body');
	if (!tbody || !assets || !assets.length) { if (tbody) tbody.innerHTML = ''; return; }
	const slice = assets.slice(start, end);
	tbody.innerHTML = slice.map(asset => assetToRowHTML(asset)).join('');
}

// renderCollectionMethodBadge surfaces how a single asset was collected
// (API, browser collection, or browser fallback after API failure) so
// the user can tell at a glance which path produced the result.
// 必须是顶层函数：被全局的 assetToRowHTML 调用，不能定义在 showResults
// 闭包内（否则 ReferenceError 导致整张表渲染失败）。
function renderCollectionMethodBadge(asset) {
	if (!asset) return '';
	const extra = asset.extra || asset.Extra || {};
	const method = String(extra.collection_method || '').toLowerCase();
	if (!method) return '';
	let cls = 'method-api';
	let label = 'API';
	if (method === 'browser') {
		cls = 'method-browser';
		label = '浏览器采集';
	} else if (method === 'browser_fallback') {
		cls = 'method-browser-fallback';
		label = 'API 失败后浏览器补采';
	}
	return ` <span class="status-badge ${cls}" title="${escapeAttr(method)}">${escapeHtml(label)}</span>`;
}

// P2-2: 单条资产转为表格行 HTML
function assetToRowHTML(asset) {
	const ip = pickGlobal(asset, 'ip', 'IP');
	const port = pickGlobal(asset, 'port', 'Port');
	const protocol = pickGlobal(asset, 'protocol', 'Protocol');
	const host = pickGlobal(asset, 'host', 'Host');
	const title = pickGlobal(asset, 'title', 'Title');
	const server = pickGlobal(asset, 'server', 'Server');
	const statusCode = pickGlobal(asset, 'status_code', 'statusCode', 'StatusCode');
	const source = pickGlobal(asset, 'source', 'Source');
	const targetURL = pickGlobal(asset, 'url', 'URL');
	const engineHref = escapeAttr(getEngineLink(source, ip));
	const methodBadge = renderCollectionMethodBadge(asset);
	return `<tr data-ip="${escapeAttr(ip)}" data-port="${escapeAttr(port)}" data-protocol="${escapeAttr(protocol)}" data-host="${escapeAttr(host)}" data-title="${escapeAttr(title)}" data-server="${escapeAttr(server)}" data-status="${escapeAttr(statusCode)}" data-source="${escapeAttr(source)}" data-url="${escapeAttr(targetURL)}">
		<td>${escapeHtml(ip)}</td><td>${escapeHtml(port)}</td><td>${escapeHtml(protocol)}</td><td>${escapeHtml(host)}</td><td>${escapeHtml(title)}</td><td>${escapeHtml(server)}</td><td>${escapeHtml(statusCode)}</td><td>${escapeHtml(source)}${methodBadge}</td>
		<td><button type="button" class="btn btn-sm btn-info btn-detail" data-ip="${escapeAttr(ip)}" data-port="${escapeAttr(port)}">详情</button> <button type="button" class="btn btn-sm btn-success btn-copy" data-ip="${escapeAttr(ip)}">复制IP</button> <a href="${engineHref}" target="_blank" class="btn btn-sm btn-primary">跳转</a> <button type="button" class="btn btn-sm btn-warning btn-screenshot" data-url="${escapeAttr(targetURL)}" data-ip="${escapeAttr(ip)}" data-port="${escapeAttr(port)}" data-protocol="${escapeAttr(protocol)}">截图</button></td>
	</tr>`;
}

// P2-2: 全局 pick 函数（供 renderAssetRows 使用）
function pickGlobal(obj, ...keys) {
	if (!obj) return '';
	for (const key of keys) {
		if (obj[key] !== undefined && obj[key] !== null) return obj[key];
	}
	return '';
}


// 处理工具栏操作
function handleToolbarAction(action) {
	const queryInput = document.getElementById('query');
	
	switch (action) {
		case 'clear':
			queryInput.value = '';
			queryInput.focus();
			break;
		case 'format':
			formatQuery(queryInput);
			break;
		case 'history':
			openQueryHistory();
			break;
	}
}

// 格式化查询语句
function formatQuery(input) {
	const query = input.value;
	if (!query.trim()) return;
	
	// 简单的格式化逻辑
	let formatted = query
		.replace(/&&/g, ' && ')  // 在&&前后添加空格
		.replace(/\|\|/g, ' || ')  // 在||前后添加空格
		.replace(/!=/g, ' != ')  // 在!=前后添加空格
		.replace(/=~/g, ' =~ ')  // 在=~前后添加空格
		.replace(/=/g, ' = ')  // 在=前后添加空格
		.replace(/\s+/g, ' ')  // 多个空格替换为单个空格
		.trim();
	
	input.value = formatted;
}

// 打开查询历史
function openQueryHistory() {
	const modal = document.getElementById('query-history');
	const historyList = document.getElementById('history-list');
	
	// 清空历史记录列表
	historyList.innerHTML = '<li class="no-history">加载中...</li>';
	
	// 显示模态框
	modal.style.display = 'block';
	
	// 从服务端获取历史记录
	apiFetch('/api/v1/history?type=query&limit=50')
		.then(parseJsonResponse)
		.then(data => {
			historyList.innerHTML = '';
			const items = data.items || [];
			
			if (items.length === 0) {
				historyList.innerHTML = '<li class="no-history">无查询历史</li>';
			} else {
				items.forEach(item => {
					const li = document.createElement('li');
					const codeEl = document.createElement('code');
					// 解析 input JSON 获取查询语句
					let queryText = '';
					try {
						const input = JSON.parse(item.input);
						queryText = input.query || item.input;
					} catch (e) {
						queryText = item.input;
					}
					codeEl.textContent = queryText;
					const timeEl = document.createElement('small');
					timeEl.textContent = new Date(item.created_at).toLocaleString();
					const statusEl = document.createElement('span');
					statusEl.className = 'status-badge status-' + item.status;
					statusEl.textContent = item.status === 'success' ? '成功' : item.status === 'partial' ? '部分' : '失败';
					li.appendChild(codeEl);
					li.appendChild(statusEl);
					li.appendChild(timeEl);
					li.addEventListener('click', function() {
						const queryInput = document.getElementById('query');
						queryInput.value = queryText;
						queryInput.focus();
						closeQueryHistory();
					});
					historyList.appendChild(li);
				});
			}
		})
		.catch(err => {
			historyList.innerHTML = '<li class="no-history">加载失败</li>';
			console.error('Failed to load history:', err);
		});
	
	// 关闭按钮事件
	const closeBtns = modal.querySelectorAll('.close-btn');
	closeBtns.forEach(btn => {
		btn.addEventListener('click', closeQueryHistory);
	});
	
	// 清空历史按钮
	const clearBtn = document.getElementById('btn-clear-history');
	if (clearBtn) {
		clearBtn.addEventListener('click', function() {
			if (confirm('确定要清空所有查询历史吗？')) {
				apiFetch('/api/v1/history?type=query', { method: 'DELETE' })
					.then(() => {
						historyList.innerHTML = '<li class="no-history">无查询历史</li>';
					})
					.catch(err => console.error('Failed to clear history:', err));
			}
		});
	}
}

// 关闭查询历史
function closeQueryHistory() {
	const modal = document.getElementById('query-history');
	modal.style.display = 'none';
}

// 保存查询到历史记录
function saveQueryToHistory(query) {
	if (!query.trim()) return;
	
	const history = getQueryHistory();
	
	// 检查是否已存在相同查询
	const existingIndex = history.findIndex(item => item.query === query);
	if (existingIndex !== -1) {
		// 移除旧记录
		history.splice(existingIndex, 1);
	}
	
	// 添加新记录到开头
	history.unshift({
		query: query,
		timestamp: Date.now()
	});
	
	// 限制历史记录数量
	const maxHistory = 20;
	if (history.length > maxHistory) {
		history.splice(maxHistory);
	}
	
	// 保存到本地存储
	localStorage.setItem('queryHistory', JSON.stringify(history));
}

// 获取查询历史
function getQueryHistory() {
	try {
		const history = localStorage.getItem('queryHistory');
		return history ? JSON.parse(history) : [];
	} catch (e) {
		console.error('获取查询历史失败:', e);
		return [];
	}
}

// 清空查询历史
function clearQueryHistory() {
	localStorage.removeItem('queryHistory');
}

// 保存查询到服务端历史
function saveQueryToServerHistory(query, engines, data) {
	if (!query || !query.trim()) return;
	
	const input = { query: query, engines: engines };
	const summary = data.engineStats || data.EngineStats || {};
	const assets = (data.assets || data.Assets || []);
	const results = assets.slice(0, 1000).map(a => ({
		ip: a.ip || a.IP || '',
		port: a.port || a.Port || 0,
		protocol: a.protocol || a.Protocol || '',
		host: a.host || a.Host || '',
		url: a.url || a.URL || '',
		title: a.title || a.Title || '',
		server: a.server || a.Server || '',
		status_code: a.status_code || a.StatusCode || 0,
		country_code: a.country_code || a.CountryCode || '',
		source: a.source || a.Source || ''
	}));
	
	const errors = data.errors || data.Errors || [];
	const status = errors.length > 0 ? 'partial' : 'success';
	
	apiFetch('/api/v1/history/save', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			operation_type: 'query',
			input: input,
			status: status,
			total_count: data.totalCount || data.TotalCount || 0,
			summary: summary,
			results: results
		})
	}).catch(err => console.error('Failed to save query history:', err));
}

// 保存查询
function saveQuery() {
	const queryInput = document.getElementById('query');
	const query = queryInput.value;
	
	if (!query.trim()) {
		alert('请输入查询语句');
		return;
	}
	
	// 保存到本地存储
	const savedQueries = getSavedQueries();
	const queryName = prompt('请输入查询名称:');
	
	if (queryName) {
		savedQueries.push({
			name: queryName,
			query: query,
			timestamp: Date.now()
		});
		
		localStorage.setItem('savedQueries', JSON.stringify(savedQueries));
		showMessage('查询保存成功', 'success');
	}
}

// 获取保存的查询
function getSavedQueries() {
	try {
		const saved = localStorage.getItem('savedQueries');
		return saved ? JSON.parse(saved) : [];
	} catch (e) {
		console.error('获取保存的查询失败:', e);
		return [];
	}
}

// 检查引擎状态
function checkEngineStatus() {
	apiFetch('/api/v1/config')
		.then(parseJsonResponse)
		.then(data => {
			if (!data.engines) return;
			let anyHasKey = false;
			Object.entries(data.engines).forEach(([name, eng]) => {
				const el = document.getElementById(`apikey-${name}`);
				const hasKey = !!(eng.api_key && eng.api_key !== '****');
				const enabled = eng.enabled !== false;
				engineStatusMap[name] = { hasKey, enabled };
				if (hasKey) anyHasKey = true;
				if (!el) return;
				if (enabled && hasKey) {
					el.textContent = 'Key ✓';
					el.setAttribute('data-state', 'ok');
					el.title = 'API Key 已配置';
				} else if (enabled) {
					el.textContent = 'Key ✗';
					el.setAttribute('data-state', 'warn');
					el.title = '未配置 API Key';
				} else {
					el.textContent = 'Key ✗';
					el.setAttribute('data-state', 'error');
					el.title = '引擎已禁用';
				}
			});
			// 自动取消勾选没有 API Key 的引擎，避免 Shodan 等 Web-only 适配器被误选进 API 查询。
			document.querySelectorAll('input[name="engines"]').forEach(function(cb) {
				var st = engineStatusMap[cb.value];
				if (st && !st.hasKey) {
					cb.checked = false;
				}
			});
			// P1-1: 首次使用引导 — 无任何引擎配置时显示提示
			updateFirstUseBanner(anyHasKey);
		})
		.catch(function(err) { console.warn('checkEngineStatus failed:', err); });
}

// 检查各引擎登录状态（页面加载时调用）
function checkLoginStatus() {
	apiFetch('/api/v1/cookies/login-status')
		.then(parseJsonResponse)
		.then(data => {
			if (!data.engines) return;
			data.engines.forEach(function(item) {
				var name = item.engine;
				var loggedIn = !!item.logged_in;
				var reason = item.reason || '';
				var loginURL = item.login_url || '';
				var el = document.getElementById('login-' + name);
				if (!el) return;

				if (loggedIn || reason === 'ext_connected' || reason === 'cdp_connected') {
					el.textContent = '●';
					el.setAttribute('data-state', 'ok');
					el.title = '已连接';
				} else if (reason === 'cookie_configured') {
					el.textContent = '●';
					el.setAttribute('data-state', 'warn');
					el.title = 'Cookie 已配置，但未检测到登录状态';
				} else {
					el.textContent = '○';
					el.setAttribute('data-state', 'error');
					el.title = '未连接' + (loginURL ? '\n' + loginURL : '');
				}
			});
		})
		.catch(function(err) { console.warn('checkLoginStatus failed:', err); });
}

// 首次使用引导 banner
function updateFirstUseBanner(anyHasKey) {
	let banner = document.getElementById('first-use-banner');
	if (anyHasKey) {
		if (banner) banner.style.display = 'none';
		return;
	}
	if (!banner) {
		banner = document.createElement('div');
		banner.id = 'first-use-banner';
		banner.style.cssText = 'background:#fff3cd; border:1px solid #ffc107; color:#856404; padding:12px 16px; border-radius:6px; margin-bottom:16px; font-size:14px;';
		banner.innerHTML = '⚠️ 尚未配置任何引擎 API Key，请先 <a href="/settings#panel-engines" style="color:#0056b3; font-weight:bold;">前往设置</a> 添加引擎密钥后再进行查询。';
		const form = document.getElementById('query-form');
		if (form && form.parentNode) {
			form.parentNode.insertBefore(banner, form);
		}
	}
	banner.style.display = 'block';
}

// 初始化结果表格
function initResultsTable() {
	const table = document.querySelector('.results-table');
	if (!table) return;

	// P2-2: 表格排序功能（排序数据数组，重新渲染当前页）
	const sortKeys = ['ip', 'port', 'protocol', 'host', 'title', 'server', 'status_code', 'source'];
	const headers = table.querySelectorAll('th');
	headers.forEach((header, idx) => {
		if (idx >= sortKeys.length) return;
		header.addEventListener('click', function() {
			const key = sortKeys[idx];
			const altKeys = { ip: 'IP', port: 'Port', protocol: 'Protocol', host: 'Host', title: 'Title', server: 'Server', status_code: 'StatusCode', source: 'Source' };
			const isAscending = this.classList.contains('sort-asc');
			this.classList.toggle('sort-asc', !isAscending);
			this.classList.toggle('sort-desc', isAscending);

			currentAssets.sort((a, b) => {
				const av = String(a[key] || a[altKeys[key]] || '');
				const bv = String(b[key] || b[altKeys[key]] || '');
				if (!isNaN(av) && !isNaN(bv) && av !== '' && bv !== '') {
					return isAscending ? parseFloat(av) - parseFloat(bv) : parseFloat(bv) - parseFloat(av);
				}
				return isAscending ? av.localeCompare(bv) : bv.localeCompare(av);
			});
			// 重新渲染第一页
			const pageSize = parseInt(document.getElementById('page-size')?.value) || 50;
			renderAssetRows(0, pageSize);
		});
	});
	
	// 初始化筛选功能
	initFilterOptions();
	
	// 初始化分页功能
	initPagination();
	
	// 初始化资产详情功能
	initAssetDetail();
	
	// 初始化导出功能
	initExportButtons();
}

// 初始化导出按钮
function initExportButtons() {
	const csvBtn = document.getElementById('btn-export-csv');
	const jsonBtn = document.getElementById('btn-export-json');
	const excelBtn = document.getElementById('btn-export-excel');
	
	if (csvBtn) {
		csvBtn.addEventListener('click', function() {
			exportToCSV();
		});
	}
	
	if (jsonBtn) {
		jsonBtn.addEventListener('click', function() {
			exportToJSON();
		});
	}
	
	if (excelBtn) {
		excelBtn.addEventListener('click', function() {
			exportToExcel();
		});
	}
}

// 导出为CSV
function exportToCSV() {
	const assets = getExportAssets();
	if (assets.length === 0) {
		showMessage('没有可导出的结果', 'warning');
		return;
	}
	const columns = getExportColumns();
	let csvContent = columns.map(col => csvEscape(col.label)).join(',') + '\n';
	assets.forEach(asset => {
		csvContent += columns.map(col => csvEscape(pickGlobal(asset, ...col.keys))).join(',') + '\n';
	});

	// 创建下载链接
	const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
	const link = document.createElement('a');
	const url = URL.createObjectURL(blob);
	link.setAttribute('href', url);
	link.setAttribute('download', `unimap_results_${new Date().toISOString().slice(0, 10)}.csv`);
	link.style.visibility = 'hidden';
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
	showMessage('CSV导出成功', 'success');
}

// 导出为JSON
function exportToJSON() {
	const assets = getExportAssets();
	if (assets.length === 0) {
		showMessage('没有可导出的结果', 'warning');
		return;
	}
	const columns = getExportColumns();
	const jsonData = assets.map(asset => {
		const row = {};
		columns.forEach(col => {
			row[col.label] = pickGlobal(asset, ...col.keys);
		});
		return row;
	});

	// 构建完整的JSON对象
	const exportData = {
		timestamp: new Date().toISOString(),
		resultCount: jsonData.length,
		data: jsonData
	};
	
	// 创建下载链接
	const jsonString = JSON.stringify(exportData, null, 2);
	const blob = new Blob([jsonString], { type: 'application/json;charset=utf-8;' });
	const link = document.createElement('a');
	const url = URL.createObjectURL(blob);
	link.setAttribute('href', url);
	link.setAttribute('download', `unimap_results_${new Date().toISOString().slice(0, 10)}.json`);
	link.style.visibility = 'hidden';
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
	showMessage('JSON导出成功', 'success');
}

function getExportAssets() {
	return filteredAssets || currentAssets || [];
}

function getExportColumns() {
	return [
		{ label: 'IP', keys: ['ip', 'IP'] },
		{ label: '端口', keys: ['port', 'Port'] },
		{ label: '协议', keys: ['protocol', 'Protocol'] },
		{ label: '主机', keys: ['host', 'Host'] },
		{ label: '标题', keys: ['title', 'Title'] },
		{ label: '服务器', keys: ['server', 'Server'] },
		{ label: '状态码', keys: ['status_code', 'statusCode', 'StatusCode'] },
		{ label: '来源', keys: ['source', 'Source'] },
		{ label: 'URL', keys: ['url', 'URL'] },
	];
}

function csvEscape(value) {
	const s = String(value ?? '');
	return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

// 导出为Excel（实际上是CSV）
function exportToExcel() {
	// 显示提示信息
	showMessage('Excel导出功能使用CSV格式，正在导出...', 'info');
	
	// 延迟执行CSV导出
	setTimeout(() => {
		exportToCSV();
	}, 500);
}

// 初始化筛选选项
function initFilterOptions() {
	const applyFilterBtn = document.getElementById('btn-apply-filter');
	const resetFilterBtn = document.getElementById('btn-reset-filter');
	
	if (applyFilterBtn) {
		applyFilterBtn.addEventListener('click', applyFilters);
	}
	
	if (resetFilterBtn) {
		resetFilterBtn.addEventListener('click', resetFilters);
	}
}

// 应用筛选（P2-2: 筛选数据数组，重新渲染）
function applyFilters() {
	const ipFilter = document.getElementById('filter-ip').value.toLowerCase();
	const portFilter = document.getElementById('filter-port').value.toLowerCase();
	const protocolFilter = document.getElementById('filter-protocol').value.toLowerCase();
	const sourceFilter = document.getElementById('filter-source').value.toLowerCase();

	filteredAssets = currentAssets.filter(asset => {
		const ip = String(asset.ip || asset.IP || '').toLowerCase();
		const port = String(asset.port || asset.Port || '').toLowerCase();
		const protocol = String(asset.protocol || asset.Protocol || '').toLowerCase();
		const source = String(asset.source || asset.Source || '').toLowerCase();
		return (!ipFilter || ip.includes(ipFilter))
			&& (!portFilter || port.includes(portFilter))
			&& (!protocolFilter || protocol.includes(protocolFilter))
			&& (!sourceFilter || source.includes(sourceFilter));
	});

	const displayedCountElement = document.getElementById('displayed-count');
	if (displayedCountElement) displayedCountElement.textContent = filteredAssets.length;

	const pageSize = parseInt(document.getElementById('page-size')?.value) || 50;
	renderAssetRowsFrom(filteredAssets, 0, pageSize);
}

// 重置筛选（P2-2: 重置数据数组，重新渲染）
function resetFilters() {
	document.getElementById('filter-ip').value = '';
	document.getElementById('filter-port').value = '';
	document.getElementById('filter-protocol').value = '';
	document.getElementById('filter-source').value = '';
	filteredAssets = null;

	const displayedCountElement = document.getElementById('displayed-count');
	if (displayedCountElement) displayedCountElement.textContent = currentAssets.length;

	const pageSize = parseInt(document.getElementById('page-size')?.value) || 50;
	renderAssetRows(0, pageSize);
}

// 初始化分页功能（P2-2: 按页渲染，仅当前页在 DOM 中）
function initPagination() {
	const prevBtn = document.getElementById('btn-prev-page');
	const nextBtn = document.getElementById('btn-next-page');
	const pageSizeSelect = document.getElementById('page-size');
	if (!currentAssets || currentAssets.length === 0) return;

	let currentPage = 1;
	let pageSize = parseInt(pageSizeSelect?.value) || 50;

	function getActiveAssets() { return filteredAssets || currentAssets; }

	function getTotalPages() { return Math.max(1, Math.ceil(getActiveAssets().length / pageSize)); }

	function renderPage() {
		const assets = getActiveAssets();
		const start = (currentPage - 1) * pageSize;
		const end = start + pageSize;
		renderAssetRowsFrom(assets, start, end);
		if (prevBtn) prevBtn.disabled = currentPage <= 1;
		if (nextBtn) nextBtn.disabled = currentPage >= getTotalPages();
		const cp = document.getElementById('current-page');
		const tp = document.getElementById('total-pages');
		if (cp) cp.textContent = currentPage;
		if (tp) tp.textContent = getTotalPages();
	}

	if (prevBtn) {
		prevBtn.addEventListener('click', function() {
			if (currentPage > 1) { currentPage--; renderPage(); }
		});
	}
	if (nextBtn) {
		nextBtn.addEventListener('click', function() {
			if (currentPage < getTotalPages()) { currentPage++; renderPage(); }
		});
	}
	if (pageSizeSelect) {
		pageSizeSelect.addEventListener('change', function() {
			pageSize = parseInt(this.value) || 50;
			currentPage = 1;
			renderPage();
		});
	}

	// 第一页已在 showResults 中渲染，无需重复
	if (prevBtn) prevBtn.disabled = true;
	if (nextBtn) nextBtn.disabled = getTotalPages() <= 1;
	const cp = document.getElementById('current-page');
	const tp = document.getElementById('total-pages');
	if (cp) cp.textContent = 1;
	if (tp) tp.textContent = getTotalPages();
}

// 事件委托：资产详情/复制/截图按钮（仅绑定一次，翻页不影响）
function initAssetActionDelegation() {
	var tbody = document.getElementById('results-body');
	if (!tbody || tbody.dataset.assetDelegated === '1') return;
	tbody.dataset.assetDelegated = '1';

	tbody.addEventListener('click', function(e) {
		var btn = e.target.closest('button');
		if (!btn) return;

		if (btn.classList.contains('btn-detail')) {
			var ip = btn.getAttribute('data-ip');
			var port = btn.getAttribute('data-port');
			showAssetDetail(ip, port);
		} else if (btn.classList.contains('btn-copy')) {
			var ip = btn.getAttribute('data-ip');
			if (!ip) return;
			copyToClipboard(ip)
				.then(function() { showMessage('IP地址已复制到剪贴板', 'success'); })
				.catch(function(err) { console.error('复制失败:', err); fallbackCopy(ip); });
		} else if (btn.classList.contains('btn-screenshot')) {
			var url = btn.getAttribute('data-url');
			var ip = btn.getAttribute('data-ip');
			var port = btn.getAttribute('data-port');
			var proto = btn.getAttribute('data-protocol');
			viewScreenshot(url, ip, port, proto);
		}
	});
}

function fallbackCopy(text) {
	const textArea = document.createElement("textarea");
	textArea.value = text;
	document.body.appendChild(textArea);
	textArea.focus();
	textArea.select();
	try {
		const successful = document.execCommand('copy');
		if (successful) {
			showMessage('IP地址已复制到剪贴板', 'success');
		} else {
			showMessage('复制失败，请手动复制', 'error');
		}
	} catch (err) {
		showMessage('复制失败', 'error');
	}
	document.body.removeChild(textArea);
}

// 显示资产详情
function showAssetDetail(ip, port) {
	const modal = document.getElementById('asset-detail');
	const content = document.getElementById('asset-detail-content');
	if (!modal || !content) return;

	// 从表格行读取真实数据
	const row = document.querySelector(`tr[data-ip="${ip}"][data-port="${port}"]`);
	const d = row ? row.dataset : {};

	const fields = [
		['IP地址', ip],
		['端口', port],
		['协议', d.protocol],
		['主机', d.host],
		['标题', d.title],
		['服务器', d.server],
		['状态码', d.status],
		['来源', d.source],
		['国家', d.country],
		['地区', d.region],
		['城市', d.city],
		['ASN', d.asn],
		['组织', d.org],
		['ISP', d.isp],
		['URL', d.url],
	];

	content.innerHTML = fields.filter(f => f[1] && f[1] !== '0' && f[1] !== 'undefined').map(f => `
		<div class="asset-detail-item">
			<span class="asset-detail-label">${f[0]}：</span>
			<span class="asset-detail-value"><code>${escapeHtml(f[1])}</code></span>
		</div>
	`).join('');

	modal.style.display = 'block';

	function closeModal() {
		modal.style.display = 'none';
		window.removeEventListener('click', handleOutsideClick);
	}
	function handleOutsideClick(e) {
		if (e.target === modal) {
			closeModal();
		}
	}
	modal.querySelectorAll('.close-btn').forEach(function(btn) {
		btn.onclick = closeModal;
	});
	window.addEventListener('click', handleOutsideClick);
}

// 初始化配额页面
function initQuotaPage() {
	const quotaGrid = document.querySelector('.quota-grid');
	if (!quotaGrid) return;

	// Set per-engine status based on rendered content
	quotaGrid.querySelectorAll('.quota-item').forEach(item => {
		const status = item.querySelector('.quota-status');
		const errText = (item.querySelector('.quota-error')?.textContent || '').trim();
		const hasDetails = !!item.querySelector('.quota-details');
		if (!status) return;
		if (errText) {
			status.textContent = '异常';
			status.classList.add('error');
		} else if (hasDetails) {
			status.textContent = '正常';
		} else {
			status.textContent = '未知';
			status.classList.add('warning');
		}
	});
	
	// 刷新配额按钮
	const refreshBtn = document.getElementById('btn-refresh-quota');
	if (refreshBtn) {
		refreshBtn.addEventListener('click', function() {
			// 显示加载状态
			const originalText = this.textContent;
			this.textContent = '刷新中...';
			this.disabled = true;
			
			// 模拟刷新
			setTimeout(() => {
				// 重新加载页面
				location.reload();
			}, 1000);
		});
	}
	
	// 导出配额按钮
	const exportBtn = document.getElementById('btn-export-quota');
	if (exportBtn) {
		exportBtn.addEventListener('click', function() {
			exportQuota();
		});
	}
	
	// 配额设置按钮
	const settingsBtn = document.getElementById('btn-quota-settings');
	if (settingsBtn) {
		settingsBtn.addEventListener('click', function() {
			openQuotaSettings();
		});
	}
	
	// 初始化配额概览
	initQuotaOverview();
	
	// 初始化配额趋势图
	initQuotaTrend();
	
	// 初始化配额预警
	initQuotaAlert();
}

// 初始化配额概览
function initQuotaOverview() {
	const quotaItems = document.querySelectorAll('.quota-item');
	let totalRemaining = 0;
	let totalUsed = 0;
	let totalQuota = 0;

	const parseNumber = (text) => {
		if (!text) return 0;
		const cleaned = String(text).replace(/,/g, '').match(/\d+(?:\.\d+)?/);
		return cleaned ? parseFloat(cleaned[0]) : 0;
	};

	quotaItems.forEach(item => {
		let remaining = 0;
		let used = 0;
		let quota = 0;

		const rows = item.querySelectorAll('.quota-row');
		rows.forEach(row => {
			const label = (row.querySelector('.label')?.textContent || '').trim();
			const value = (row.querySelector('.value')?.textContent || '').trim();
			if (label.includes('剩余配额')) remaining = parseNumber(value);
			else if (label.includes('已用配额')) used = parseNumber(value);
			else if (label.includes('总配额')) quota = parseNumber(value);
		});

		totalRemaining += remaining;
		totalUsed += used;
		totalQuota += quota;
	});
	
	// 更新概览数据
	const totalRemainingElement = document.getElementById('total-remaining');
	const totalUsedElement = document.getElementById('total-used');
	const totalQuotaElement = document.getElementById('total-quota');
	const totalUsageRateElement = document.getElementById('total-usage-rate');
	
	if (totalRemainingElement) totalRemainingElement.textContent = Math.round(totalRemaining);
	if (totalUsedElement) totalUsedElement.textContent = Math.round(totalUsed);
	if (totalQuotaElement) totalQuotaElement.textContent = Math.round(totalQuota);
	if (totalUsageRateElement && totalQuota > 0) {
		totalUsageRateElement.textContent = `${((totalUsed / totalQuota) * 100).toFixed(1)}%`;
	}
}

// 初始化配额趋势图
function initQuotaTrend() {
	const chartContainer = document.getElementById('quota-trend-chart');
	if (!chartContainer) return;

	chartContainer.innerHTML = '<div class="chart-placeholder">配额趋势图暂未实现（当前页面展示的是实时配额数据）</div>';
}

// 初始化配额预警
function initQuotaAlert() {
	const alertCheckboxes = document.querySelectorAll('input[name="quota-alert"]');
	alertCheckboxes.forEach(checkbox => {
		checkbox.addEventListener('change', function() {
			const thresholdInput = this.parentElement.parentElement.querySelector('input[name="alert-threshold"]');
			if (thresholdInput) {
				thresholdInput.disabled = !this.checked;
			}
		});
	});
	
	// 检查使用率，更新状态
	const quotaItems = document.querySelectorAll('.quota-item');
	quotaItems.forEach(item => {
		const usageRateElement = item.querySelector('.quota-row:nth-child(4) .value');
		const statusElement = item.querySelector('.quota-status');
		const progressElement = item.querySelector('.quota-progress');
		
		if (usageRateElement && statusElement && progressElement) {
			const usageRateText = usageRateElement.textContent;
			const usageRate = parseInt(usageRateText);
			
			if (usageRate >= 90) {
				statusElement.textContent = '紧急';
				statusElement.className = 'quota-status error';
				progressElement.className = 'quota-progress error';
			} else if (usageRate >= 70) {
				statusElement.textContent = '警告';
				statusElement.className = 'quota-status warning';
				progressElement.className = 'quota-progress warning';
			}
		}
	});
}

// 导出配额
function exportQuota() {
	// 模拟导出功能
	showMessage('配额数据导出中...', 'info');
	
	setTimeout(() => {
		// 创建CSV内容
		let csvContent = "引擎,剩余配额,已用配额,总配额,使用率,过期时间\n";
		
		const quotaItems = document.querySelectorAll('.quota-item');
		quotaItems.forEach(item => {
			const engineName = item.querySelector('h3').textContent;
			const rows = item.querySelectorAll('.quota-row');
			let remaining = '';
			let used = '';
			let total = '';
			let usage = '';
			let expiry = '';
			
			rows.forEach(row => {
				const label = row.querySelector('.label').textContent;
				const value = row.querySelector('.value').textContent;
				
				if (label.includes('剩余配额')) {
					remaining = value;
				} else if (label.includes('已用配额')) {
					used = value;
				} else if (label.includes('总配额')) {
					total = value;
				} else if (label.includes('使用率')) {
					usage = value;
				} else if (label.includes('过期时间')) {
					expiry = value;
				}
			});
			
			csvContent += `${engineName},${remaining},${used},${total},${usage},${expiry}\n`;
		});
		
		// 创建下载链接
		const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
		const link = document.createElement('a');
		const url = URL.createObjectURL(blob);
		link.setAttribute('href', url);
		link.setAttribute('download', `quota_${new Date().toISOString().slice(0, 10)}.csv`);
		link.style.visibility = 'hidden';
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		URL.revokeObjectURL(url);
		showMessage('配额数据导出成功', 'success');
	}, 1000);
}

// 打开配额设置
function openQuotaSettings() {
	const modal = document.getElementById('quota-settings-modal');
	if (!modal) return;

	// 加载已保存的设置
	try {
		const saved = JSON.parse(localStorage.getItem('unimap_quota_settings') || '{}');
		const ri = modal.querySelector('[name="refresh-interval"]');
		const dt = modal.querySelector('[name="default-threshold"]');
		const ea = modal.querySelector('[name="email-alert"]');
		const em = modal.querySelector('[name="alert-email"]');
		if (ri && saved.refreshInterval) ri.value = saved.refreshInterval;
		if (dt && saved.defaultThreshold) dt.value = saved.defaultThreshold;
		if (ea) ea.checked = !!saved.emailAlert;
		if (em && saved.alertEmail) em.value = saved.alertEmail;
	} catch(e) {}

	modal.style.display = 'block';

	function closeQuotaModal() {
		modal.style.display = 'none';
		window.removeEventListener('click', handleOutsideClick);
	}
	function handleOutsideClick(e) {
		if (e.target === modal) {
			closeQuotaModal();
		}
	}
	modal.querySelectorAll('.close-btn').forEach(function(btn) {
		btn.onclick = closeQuotaModal;
	});

	const saveBtn = document.getElementById('btn-save-settings');
	if (saveBtn) {
		saveBtn.onclick = function() {
			const settings = {
				refreshInterval: modal.querySelector('[name="refresh-interval"]')?.value || '60000',
				defaultThreshold: modal.querySelector('[name="default-threshold"]')?.value || '80',
				emailAlert: modal.querySelector('[name="email-alert"]')?.checked || false,
				alertEmail: modal.querySelector('[name="alert-email"]')?.value || '',
			};
			try {
				localStorage.setItem('unimap_quota_settings', JSON.stringify(settings));
				showMessage('设置已保存', 'success');
			} catch(e) {
				showMessage('保存失败: ' + e.message, 'error');
			}
			modal.style.display = 'none';
		};
	}

	window.addEventListener('click', handleOutsideClick);
}

// 工具函数：格式化数字
function formatNumber(num) {
	return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

// 工具函数：格式化时间
function formatDate(dateString) {
	const date = new Date(dateString);
	return date.toLocaleString();
}

// 工具函数：复制到剪贴板
function copyToClipboard(text) {
	if (navigator.clipboard && window.isSecureContext) {
		return navigator.clipboard.writeText(text);
	} else {
		const textArea = document.createElement('textarea');
		textArea.value = text;
		textArea.style.position = 'fixed';
		textArea.style.left = '-999999px';
		textArea.style.top = '-999999px';
		document.body.appendChild(textArea);
		textArea.focus();
		textArea.select();
		const result = document.execCommand('copy');
		document.body.removeChild(textArea);
		return result ? Promise.resolve() : Promise.reject(new Error('复制失败'));
	}
}

// 添加复制按钮到代码块
function addCopyButtons() {
	const codeBlocks = document.querySelectorAll('code');
	codeBlocks.forEach(codeBlock => {
		const copyBtn = document.createElement('button');
		copyBtn.textContent = '复制';
		copyBtn.className = 'copy-btn';
		copyBtn.style.position = 'absolute';
		copyBtn.style.top = '0.5rem';
		copyBtn.style.right = '0.5rem';
		copyBtn.style.padding = '0.2rem 0.5rem';
		copyBtn.style.fontSize = '0.8rem';
		copyBtn.style.backgroundColor = 'rgba(255,255,255,0.8)';
		copyBtn.style.border = '1px solid #ddd';
		copyBtn.style.borderRadius = '3px';
		copyBtn.style.cursor = 'pointer';
		
		const parent = codeBlock.parentElement;
		parent.style.position = 'relative';
		parent.appendChild(copyBtn);
		
		copyBtn.addEventListener('click', function() {
			copyToClipboard(codeBlock.textContent.trim())
				.then(() => {
					this.textContent = '已复制';
					setTimeout(() => {
						this.textContent = '复制';
					}, 2000);
				})
				.catch(err => {
					console.error('复制失败:', err);
					this.textContent = '复制失败';
					setTimeout(() => {
						this.textContent = '复制';
					}, 2000);
				});
		});
	});
}

// 平滑滚动到指定元素
function scrollToElement(elementId) {
	const element = document.getElementById(elementId);
	if (element) {
		element.scrollIntoView({ behavior: 'smooth' });
	}
}

// 显示消息提示
function handleBrowserError(err) {
	const s = typeof err === 'string' ? err : (err && err.message ? err.message : '');
	if (!s) return false;
	const lower = s.toLowerCase();
	let msg = null;
	if (lower.includes('cdp not available') || lower.includes('invalid_chrome_path')) {
		msg = 'Chrome 浏览器不可用，截图功能暂不可用。请检查 Chrome 是否已安装。';
	} else if (lower.includes('extension_not_paired') || lower.includes('extension not paired')) {
		msg = '浏览器扩展未连接，截图功能暂不可用。请检查扩展是否已安装并运行。';
	} else if (lower.includes('bridge_unavailable') || lower.includes('bridge unavailable')) {
		msg = '浏览器桥接服务不可用，扩展截图功能暂不可用。请检查桥接服务状态。';
	} else if (lower.includes('screenshot_failed') && lower.includes('chrome')) {
		msg = 'Chrome 浏览器不可用，截图功能暂不可用。请检查 Chrome 是否已安装。';
	} else if (lower.includes('screenshot_failed') || lower.includes('cdp') || lower.includes('bridge') || lower.includes('browser') || lower.includes('extension')) {
		msg = '浏览器服务暂不可用，请稍后重试。';
	}
	if (msg) {
		showMessage(msg, 'warning', 5000);
		return true;
	}
	return false;
}

function showMessage(message, type = 'info', duration = 3000) {
	const messageDiv = document.createElement('div');
	messageDiv.className = `message message-${type}`;
	messageDiv.textContent = message;
	messageDiv.style.position = 'fixed';
	messageDiv.style.top = '20px';
	messageDiv.style.right = '20px';
	messageDiv.style.padding = '1rem';
	messageDiv.style.borderRadius = '4px';
	messageDiv.style.zIndex = '1000';
	messageDiv.style.boxShadow = '0 2px 8px rgba(0,0,0,0.2)';
	messageDiv.style.transition = 'all 0.3s ease';
	
	// 设置消息类型样式
	switch (type) {
		case 'success':
			messageDiv.style.backgroundColor = '#d4edda';
			messageDiv.style.color = '#155724';
			messageDiv.style.border = '1px solid #c3e6cb';
			break;
		case 'error':
			messageDiv.style.backgroundColor = '#f8d7da';
			messageDiv.style.color = '#721c24';
			messageDiv.style.border = '1px solid #f5c6cb';
			break;
		case 'warning':
			messageDiv.style.backgroundColor = '#fff3cd';
			messageDiv.style.color = '#856404';
			messageDiv.style.border = '1px solid #ffeeba';
			break;
		default:
			messageDiv.style.backgroundColor = '#d1ecf1';
			messageDiv.style.color = '#0c5460';
			messageDiv.style.border = '1px solid #bee5eb';
	}
	
	document.body.appendChild(messageDiv);
	
	// 自动消失
	setTimeout(() => {
		messageDiv.style.opacity = '0';
		messageDiv.style.transform = 'translateX(100%)';
		setTimeout(() => {
			document.body.removeChild(messageDiv);
		}, 300);
	}, duration);
}

// 获取引擎跳转链接
function getEngineLink(source, ip) {
	if (!ip) return '#';
	const query = `ip="${ip}"`;
	// Base64 encode
	let b64 = "";
	try {
		b64 = btoa(query);
	} catch (e) {
		console.error("Base64 encode failed", e);
		return "#";
	}
	
	switch(source ? source.toLowerCase() : '') {
		case 'fofa': return `https://fofa.info/result?qbase64=${b64}`;
		case 'hunter': return `https://hunter.qianxin.com/list?searchValue=${b64}`;
		case 'quake': return `https://quake.360.net/quake/#/searchResult?searchVal=${encodeURIComponent(query)}`;
		case 'zoomeye': return `https://www.zoomeye.org/searchResult?q=${encodeURIComponent('ip:"'+ip+'"')}`;
		default: return '#';
	}
}

// 查看截图
function viewScreenshot(url, ip, port, protocol) {
	let target = url;
	if (!target || target === "undefined") {
		if (ip) {
			target = `${(protocol || "http").toLowerCase()}://${ip}:${port || 80}`;
		} else {
			alert('无法获取目标URL');
			return;
		}
	}
	
	// 创建或获取模态框
	let modal = document.getElementById('screenshot-modal');
	if (!modal) {
		modal = document.createElement('div');
		modal.id = 'screenshot-modal';
		modal.className = 'modal';
		modal.innerHTML = `
			<div class="modal-content" style="max-width:900px; width:90%;">
				<div class="modal-header">
					<h3 id="screenshot-title">目标截图</h3>
					<button type="button" class="close-btn">&times;</button>
				</div>
				<div class="modal-body" style="text-align:center; min-height:300px; display:flex; justify-content:center; align-items:center; flex-direction: column;">
					<p>正在截图，请稍候...</p>
				</div>
				<div class="modal-footer">
					<button type="button" class="btn btn-secondary close-btn">关闭</button>
					<a href="#" target="_blank" id="open-link-btn" class="btn btn-primary">访问目标</a>
				</div>
			</div>
		`;
		document.body.appendChild(modal);
		
		// 绑定关闭事件
		function closeModal() {
			// P2-1: 释放 blob URL 防止内存泄漏
			if (modal._blobUrl) { URL.revokeObjectURL(modal._blobUrl); modal._blobUrl = null; }
			modal.style.display = 'none';
		}
		modal.onclick = function(e) {
			if (e.target === modal) closeModal();
		};
		modal.querySelectorAll('.close-btn').forEach(btn => {
			btn.onclick = closeModal;
		});
	}
	
	const title = modal.querySelector('#screenshot-title');
	title.textContent = "目标截图: " + target;

	const body = modal.querySelector('.modal-body');
	const linkBtn = modal.querySelector('#open-link-btn');
	
	body.innerHTML = '<div class="spinner"></div><p style="margin-top:10px;">正在截取页面，可能需要几秒钟(视目标响应速度)...</p>';
	linkBtn.href = target;
	modal.style.display = 'block';
	
	// 请求截图 (POST)
	apiFetch('/api/v1/screenshot', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ url: target })
	})
	.then(resp => {
		if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
		return resp.blob();
	})
	.then(blob => {
		const imgUrl = URL.createObjectURL(blob);
		// P2-1: 存储 blob URL 以便关闭时释放
		const modalEl = document.getElementById('screenshot-modal');
		if (modalEl && modalEl._blobUrl) { URL.revokeObjectURL(modalEl._blobUrl); }
		if (modalEl) modalEl._blobUrl = imgUrl;
		body.innerHTML = '';
		const img = document.createElement('img');
		img.src = imgUrl;
		img.style.maxWidth = '100%';
		img.style.maxHeight = '600px';
		img.style.border = '1px solid #ddd';
		img.style.boxShadow = '0 0 10px rgba(0,0,0,0.1)';
		body.appendChild(img);
	})
	.catch((err) => {
		if (!handleBrowserError(err)) {
			body.innerHTML = `
				<div style="color:#721c24; background:#f8d7da; padding:20px; border-radius:5px;">
					<h4>截图失败</h4>
					<p>目标可能无法访问或响应超时。</p>
					<p>URL: ${escapeHtml(target)}</p>
				</div>
			`;
		}
	});
}

// 截图搜索引擎结果页面
function captureSearchEngineScreenshots() {
	if (!window.currentQueryData) {
		showMessage('没有可用的查询数据', 'warning');
		return;
	}

	const { query, engines, queryID } = window.currentQueryData;
	if (!engines || engines.length === 0) {
		showMessage('没有可用的搜索引擎', 'warning');
		return;
	}

	const statusEl = document.getElementById('screenshot-status');
	const progressEl = document.getElementById('screenshot-progress');
	const progressBar = document.getElementById('screenshot-progress-bar');
	const progressText = document.getElementById('screenshot-progress-text');

	statusEl.textContent = '正在截图搜索引擎结果页...';
	progressEl.classList.remove('hidden');

	let completed = 0;
	const total = engines.length;

	engines.forEach((engine, index) => {
		setTimeout(() => {
			apiFetch(`/api/v1/screenshot/search-engine?engine=${encodeURIComponent(engine)}&query=${encodeURIComponent(query)}&query_id=${queryID}`)
				.then(parseJsonResponse)
				.then(data => {
					completed++;
					const percent = (completed / total) * 100;
					progressBar.style.width = percent + '%';
					progressText.textContent = `已完成 ${completed}/${total}: ${engine}`;

					if (completed === total) {
						statusEl.textContent = '搜索引擎结果页截图完成!';
						showMessage('搜索引擎结果页截图完成!', 'success');
					}
				})
				.catch(err => {
					completed++;
					console.error(`截图 ${engine} 失败:`, err);
					if (completed === total) {
						statusEl.textContent = '截图完成(部分失败)';
					}
				});
		}, index * 2000); // 每个引擎间隔2秒，避免并发过高
	});
}

// 批量截图所有目标
function captureAllScreenshots() {
	if (!window.currentQueryData) {
		showMessage('没有可用的查询数据', 'warning');
		return;
	}

	const { assets, queryID, engines } = window.currentQueryData;
	if (!assets || assets.length === 0) {
		showMessage('没有可截图的目标', 'warning');
		return;
	}

	// 先截图搜索引擎结果页
	captureSearchEngineScreenshots();

	// 然后批量截图目标
	const statusEl = document.getElementById('screenshot-status');
	const progressEl = document.getElementById('screenshot-progress');
	const progressBar = document.getElementById('screenshot-progress-bar');
	const progressText = document.getElementById('screenshot-progress-text');

	statusEl.textContent = '正在批量截图目标网站...';
	progressEl.classList.remove('hidden');

	// 准备URL列表
	const urls = assets.map(asset => {
		const ip = asset.ip || asset.IP || '';
		const port = String(asset.port || asset.Port || '');
		const protocol = asset.protocol || asset.Protocol || 'http';
		const url = asset.url || asset.URL || '';

		if (url) return url;
		if (!ip) return null;

		let proto = 'http';
		if (protocol) proto = protocol.toLowerCase();
		else if (port === '443') proto = 'https';

		if (port && port !== '80' && port !== '443') {
			return `${proto}://${ip}:${port}`;
		}
		return `${proto}://${ip}`;
	}).filter(u => u);

	if (urls.length === 0) {
		statusEl.textContent = '没有有效的URL可截图';
		return;
	}

	// P1-4: 异步批量截图 — 提交后轮询进度
	const batchID = queryID || `batch_${Date.now()}`;

	apiFetch('/api/v1/screenshot/batch-urls', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			urls: urls,
			batch_id: batchID,
			concurrency: 3
		})
	})
	.then(parseJsonResponse)
	.then(data => {
		if (data.error) {
			const errMsg = extractErrorMessage(data.error, '截图失败');
			statusEl.textContent = `截图失败: ${errMsg}`;
			showMessage(errMsg, 'error');
			return;
		}

		const jobID = data.job_id;
		const total = data.total || urls.length;
		statusEl.textContent = `正在截图... (0/${total})`;
		progressBar.style.width = '0%';
		progressText.textContent = `0/${total}`;

		// 轮询进度
		let progressPollFailures = 0;
		const maxProgressPollFailures = 5;
		const pollInterval = setInterval(function() {
			apiFetch(`/api/v1/screenshot/batch/progress?job_id=${encodeURIComponent(jobID)}`)
				.then(parseJsonResponse)
				.then(job => {
					progressPollFailures = 0;
					if (job.error) return;

					const completed = job.completed || 0;
					const success = job.success || 0;
					const failed = job.failed || 0;
					const percent = total > 0 ? Math.round((completed / total) * 100) : 0;

					progressBar.style.width = percent + '%';
					progressText.textContent = `${completed}/${total} (${success} 成功, ${failed} 失败)`;
					statusEl.textContent = `正在截图... (${completed}/${total})`;

					if (job.status === 'completed' || job.status === 'failed') {
						clearInterval(pollInterval);
						if (job.status === 'completed') {
							progressBar.style.width = '100%';
							progressText.textContent = `完成: ${success} 成功, ${failed} 失败`;
							statusEl.textContent = '所有截图完成!';
							showMessage(`批量截图完成! 成功 ${success}/${total}`, success > 0 ? 'success' : 'warning');
						} else {
							statusEl.textContent = `截图失败: ${job.error || '未知错误'}`;
							showMessage(job.error || '截图任务失败', 'error');
						}
					}
				})
				.catch(function() {
					progressPollFailures++;
					if (progressPollFailures >= maxProgressPollFailures) {
						clearInterval(pollInterval);
						statusEl.textContent = '截图进度查询失败，请刷新页面';
						showMessage('截图进度查询失败，请刷新页面', 'error');
					}
				});
		}, 2000);

		// 安全超时：10 分钟后停止轮询
		setTimeout(function() {
			clearInterval(pollInterval);
			statusEl.textContent = '截图轮询超时，请手动刷新页面查看结果';
			showMessage('截图轮询超时，请手动刷新页面查看结果', 'warning');
		}, 600000);
	})
	.catch(err => {
		statusEl.textContent = `截图请求失败: ${err.message}`;
		showMessage(err.message, 'error');
	});
}

// ==================== ICP 备案查询 ====================
