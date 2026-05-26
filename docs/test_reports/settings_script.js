
(function() {
	document.querySelectorAll('.ctrl-nav-item').forEach(function(btn) {
		btn.addEventListener('click', function() {
			document.querySelectorAll('.ctrl-nav-item').forEach(function(b) { b.classList.remove('active'); });
			document.querySelectorAll('.ctrl-panel').forEach(function(p) { p.classList.remove('active'); });
			btn.classList.add('active');
			var panel = document.getElementById('panel-' + btn.getAttribute('data-panel'));
			if (panel) panel.classList.add('active');
		});
	});

	// ============================================================
	// Function definitions (must come before event listener wiring)
	// ============================================================

	async function loadConfig() {
		try {
			var resp = await fetch('/api/config', { headers: { 'Authorization': 'Bearer ' + tok() } });
			if (!resp.ok) throw new Error('Failed to load config');
			var data = await resp.json();

			if (data.engines) {
				Object.keys(data.engines).forEach(function(eng) {
					var c = data.engines[eng];
					var el = document.getElementById(eng + '-enabled');
					if (el) el.checked = c.enabled;
					// FOFA 特殊处理：api_base_url + web_base_url
					if (eng === 'fofa') {
						var abu = document.getElementById('fofa-api-base-url');
						if (abu) abu.value = c.api_base_url || '';
						var wbu = document.getElementById('fofa-web-base-url');
						if (wbu) wbu.value = c.web_base_url || 'https://fofa.info';
					} else {
						var u = document.getElementById(eng + '-base-url');
						if (u) u.value = c.base_url || '';
					}
					var q = document.getElementById(eng + '-qps');
					if (q) q.value = c.qps || 10;
					var t = document.getElementById(eng + '-timeout');
					if (t) t.value = c.timeout || 30;
					var e = document.getElementById(eng + '-email');
					if (e) e.value = c.email || '';
					var d = document.getElementById(eng + '-dot');
					if (d) d.className = 'dot ' + (c.enabled ? 'on' : 'off');
				});
			}
			if (data.icp) {
				var el = document.getElementById('icp-enabled');
				if (el) el.checked = data.icp.enabled;
				var u = document.getElementById('icp-base-url');
				if (u) u.value = data.icp.base_url || '';
				var t = document.getElementById('icp-timeout');
				if (t) t.value = data.icp.timeout || 30;
				var tp = document.getElementById('icp-default-type');
				if (tp) tp.value = data.icp.default_type || 'web';
			}
			if (data.screenshot) {
				var el = document.getElementById('screenshot-enabled');
				if (el) el.checked = data.screenshot.enabled;
				var m = document.getElementById('screenshot-mode');
				if (m) m.value = data.screenshot.mode || 'auto';
				var e = document.getElementById('screenshot-engine');
				if (e) e.value = data.screenshot.engine || 'cdp';
				var t = document.getElementById('screenshot-timeout');
				if (t) t.value = data.screenshot.timeout || 30;
			}
			if (data.system) {
				var mc = document.getElementById('max-concurrent');
				if (mc) mc.value = data.system.max_concurrent || 10;
				var ct = document.getElementById('cache-ttl');
				if (ct) ct.value = data.system.cache_ttl || 3600;
				var cs = document.getElementById('cache-max-size');
				if (cs) cs.value = data.system.cache_max_size || 1000;
			}
		} catch (e) { console.error('Config load error:', e); }
	}

	window.saveAllEngines = async function() {
		var engines = ['fofa', 'hunter', 'zoomeye', 'quake', 'shodan'];
		var data = {};
		engines.forEach(function(eng) {
			var engData = {
				enabled: document.getElementById(eng + '-enabled').checked,
				qps: parseInt(document.getElementById(eng + '-qps').value),
				timeout: parseInt(document.getElementById(eng + '-timeout').value)
			};
			// FOFA 特殊处理
			if (eng === 'fofa') {
				var abu = document.getElementById('fofa-api-base-url');
				if (abu) engData.api_base_url = abu.value;
			} else {
				var u = document.getElementById(eng + '-base-url');
				if (u) engData.base_url = u.value;
			}
			var ak = document.getElementById(eng + '-api-key');
			if (ak && ak.value) engData.api_key = ak.value;
			var em = document.getElementById(eng + '-email');
			if (em && em.value) engData.email = em.value;
			data[eng] = engData;
		});

		try {
			var resp = await fetch('/api/config', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify({ section: 'engines', data: data })
			});
			var result = await resp.json();
			showResult('engines-result', result.success ? 'ok' : 'fail', result.message || (result.success ? '全部保存成功' : '保存失败'));
			Object.keys(data).forEach(function(eng) {
				var d = document.getElementById(eng + '-dot');
				if (d) d.className = 'dot ' + (data[eng].enabled ? 'on' : 'off');
			});
		} catch (e) {
			showResult('engines-result', 'fail', '保存失败: ' + e.message);
		}
	};

	window.saveICP = async function() {
		try {
			var resp = await fetch('/api/config', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify({ section: 'icp', data: {
					enabled: document.getElementById('icp-enabled').checked,
					base_url: document.getElementById('icp-base-url').value,
					api_key: document.getElementById('icp-api-key').value,
					timeout: parseInt(document.getElementById('icp-timeout').value),
					default_type: document.getElementById('icp-default-type').value
				}})
			});
			var result = await resp.json();
			showResult('icp-result', result.success ? 'ok' : 'fail', result.message || (result.success ? '保存成功' : '保存失败'));
		} catch (e) { showResult('icp-result', 'fail', '保存失败: ' + e.message); }
	};

	window.saveScreenshot = async function() {
		try {
			var resp = await fetch('/api/config', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify({ section: 'screenshot', data: {
					enabled: document.getElementById('screenshot-enabled').checked,
					mode: document.getElementById('screenshot-mode').value,
					engine: document.getElementById('screenshot-engine').value,
					timeout: parseInt(document.getElementById('screenshot-timeout').value)
				}})
			});
			var result = await resp.json();
			showResult('screenshot-result', result.success ? 'ok' : 'fail', result.message || (result.success ? '保存成功' : '保存失败'));
		} catch (e) { showResult('screenshot-result', 'fail', '保存失败: ' + e.message); }
	};

	window.saveSystem = async function() {
		try {
			var resp = await fetch('/api/config', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify({ section: 'system', data: {
					max_concurrent: parseInt(document.getElementById('max-concurrent').value),
					cache_ttl: parseInt(document.getElementById('cache-ttl').value),
					cache_max_size: parseInt(document.getElementById('cache-max-size').value)
				}})
			});
			var result = await resp.json();
			showResult('system-result', result.success ? 'ok' : 'fail', result.message || (result.success ? '保存成功' : '保存失败'));
		} catch (e) { showResult('system-result', 'fail', '保存失败: ' + e.message); }
	};

	function showResult(id, type, msg) {
		var el = document.getElementById(id);
		if (!el) return;
		el.textContent = msg;
		el.className = 'cfg-result ' + type;
		el.style.display = 'inline-block';
		setTimeout(function() { el.style.display = 'none'; }, 5000);
	}

	function hideResult(id) {
		var el = document.getElementById(id);
		if (!el) return;
		el.textContent = '';
		el.style.display = 'none';
	}

	function tok() {
		return sessionStorage.getItem('authToken') || localStorage.getItem('authToken') || '';
	}

	// --- Cookie panel ---
	var ckIds = { fofa: 'ck-fofa', hunter: 'ck-hunter', zoomeye: 'ck-zoomeye', quake: 'ck-quake' };

	async function saveCookiesPanel() {
		var form = new URLSearchParams();
		Object.keys(ckIds).forEach(function(eng) {
			var el = document.getElementById(ckIds[eng]);
			if (el) form.append('cookie_' + eng, el.value);
		});
		try {
			var resp = await fetch('/api/cookies', {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'Authorization': 'Bearer ' + tok() },
				body: form.toString()
			});
			var data = await resp.json();
			showResult('cookies-result', data.success ? 'ok' : 'fail',
				data.success ? '保存成功' : (data.error || data.message || '保存失败'));
		} catch (e) { showResult('cookies-result', 'fail', '保存失败: ' + e.message); }
	}

	async function verifyCookiesPanel() {
		try {
			var resp = await fetch('/api/cookies/verify', {
				method: 'POST',
				headers: { 'Authorization': 'Bearer ' + tok() }
			});
			var data = await resp.json();
			showResult('cookies-result', data.success ? 'ok' : 'fail',
				data.success ? '验证完成' : (data.error || data.message || '验证失败'));
		} catch (e) { showResult('cookies-result', 'fail', '验证失败: ' + e.message); }
	}

	async function refreshCookieLoginStatus() {
		try {
			var resp = await fetch('/api/cookies/login-status');
			var data = await resp.json();
			var engines = (data && data.engines) || {};
			Object.keys(ckIds).forEach(function(eng) {
				var el = document.getElementById('ck-status-' + eng);
				if (!el) return;
				var st = engines[eng];
				if (!st) { el.textContent = '未知'; return; }
				el.textContent = st.logged_in ? '已登录' : '未登录';
			});
		} catch (e) { /* silent */ }
	}

	async function importCookieJSONPanel() {
		var engine = (document.getElementById('ck-import-engine') || {}).value;
		var jsonStr = (document.getElementById('ck-import-json') || {}).value;
		if (!engine || !jsonStr.trim()) {
			showResult('cookies-result', 'fail', '请选择引擎并粘贴 JSON');
			return;
		}
		var form = new URLSearchParams();
		form.append('engine', engine);
		form.append('cookie_json', jsonStr);
		try {
			var resp = await fetch('/api/cookies/import', {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'Authorization': 'Bearer ' + tok() },
				body: form.toString()
			});
			var data = await resp.json();
			showResult('cookies-result', data.success ? 'ok' : 'fail',
				data.success ? '导入成功' : (data.error || data.message || '导入失败'));
			if (data.success && data.cookieHeader) {
				var input = document.getElementById(ckIds[engine]);
				if (input) input.value = data.cookieHeader;
			}
		} catch (e) { showResult('cookies-result', 'fail', '导入失败: ' + e.message); }
	}

	var ckSave = document.getElementById('btn-ck-save');
	if (ckSave) ckSave.addEventListener('click', saveCookiesPanel);
	var ckVerify = document.getElementById('btn-ck-verify');
	if (ckVerify) ckVerify.addEventListener('click', verifyCookiesPanel);
	var ckRefresh = document.getElementById('btn-ck-refresh');
	if (ckRefresh) ckRefresh.addEventListener('click', refreshCookieLoginStatus);
	var ckImport = document.getElementById('btn-ck-import');
	if (ckImport) ckImport.addEventListener('click', importCookieJSONPanel);
	refreshCookieLoginStatus();

	// --- Session panel ---
	async function refreshCDPStatusPanel() {
		try {
			var resp = await fetch('/api/cdp/status');
			var data = await resp.json();
			var el = document.getElementById('sess-cdp-status');
			if (!el) return;
			el.textContent = data.connected ? ('已连接 ' + (data.url || '')) : '未连接';
		} catch (e) {
			var el2 = document.getElementById('sess-cdp-status');
			if (el2) el2.textContent = '检测失败';
		}
	}

	async function connectCDPPanel() {
		var proxy = (document.getElementById('sess-proxy') || {}).value || '';
		var form = new URLSearchParams();
		if (proxy) form.append('proxy_server', proxy);
		try {
			var resp = await fetch('/api/cdp/connect', {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'Authorization': 'Bearer ' + tok() },
				body: form.toString()
			});
			var data = await resp.json();
			showResult('session-result', data.success ? 'ok' : 'fail',
				data.success ? 'CDP 连接成功' : (data.error || data.message || 'CDP 连接失败'));
			refreshCDPStatusPanel();
		} catch (e) { showResult('session-result', 'fail', 'CDP 连接失败: ' + e.message); }
	}

	async function refreshBridgeStatusPanel() {
		try {
			var resp = await fetch('/api/screenshot/bridge/status');
			var data = await resp.json();
			var el = document.getElementById('sess-bridge-status');
			if (!el) return;
			var routerMode = data.router_mode || data.mode || '';
			var ext = !!(data.router_ext_healthy || data.bridge_connected);
			el.textContent = (ext ? '在线' : '离线') + (routerMode ? ' (' + routerMode + ')' : '');
		} catch (e) {
			var el2 = document.getElementById('sess-bridge-status');
			if (el2) el2.textContent = '检测失败';
		}
	}

	async function changeSessionMode() {
		var mode = (document.getElementById('sess-mode') || {}).value;
		if (!mode) return;
		try {
			var resp = await fetch('/api/screenshot/set-mode', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify({ mode: mode })
			});
			var data = await resp.json();
			showResult('session-result', data.success !== false ? 'ok' : 'fail',
				data.success !== false ? ('模式已切换到 ' + (data.mode || mode)) : (data.error || '切换失败'));
		} catch (e) { showResult('session-result', 'fail', '切换失败: ' + e.message); }
	}

	var sessConnect = document.getElementById('btn-sess-cdp-connect');
	if (sessConnect) sessConnect.addEventListener('click', connectCDPPanel);
	var sessBridge = document.getElementById('btn-sess-bridge-refresh');
	if (sessBridge) sessBridge.addEventListener('click', refreshBridgeStatusPanel);
	var sessMode = document.getElementById('sess-mode');
	if (sessMode) sessMode.addEventListener('change', changeSessionMode);
	refreshCDPStatusPanel();
	refreshBridgeStatusPanel();

	// --- Notification channel panel ---
	var notifyChannels = [];
	var editingChannelId = null;

	function loadNotifyChannelsPanel() {
		fetch('/api/notifications/channels', { headers: { 'Authorization': 'Bearer ' + tok() } })
			.then(function(r) { return r.json(); })
			.then(function(data) {
				notifyChannels = (data && data.channels) || [];
				renderNotifyChannelList();
			})
			.catch(function(e) { console.error('notify channels load error:', e); });
	}

	function renderNotifyChannelList() {
		var container = document.getElementById('notify-channel-list');
		if (!container) return;

		if (!notifyChannels.length) {
			container.innerHTML = '<div class="cfg-group"><div class="cfg-group-title">已配置渠道</div>' +
				'<div class="cfg-row"><span class="cfg-key">暂无</span><div class="cfg-val"><span class="hint">尚无通知渠道，请点击"新增渠道"</span></div></div></div>';
			return;
		}

		var typeNames = { dingtalk: '钉钉', feishu: '飞书', wecom: '企业微信', webhook: 'Webhook', log: '日志' };
		var typeBadge = { dingtalk: '#1677FF', feishu: '#3370FF', wecom: '#07C160', webhook: '#722ED1', log: '#8B8B8B' };

		var html = '<div class="cfg-group"><div class="cfg-group-title">已配置渠道 (' + notifyChannels.length + ')</div>';
		for (var i = 0; i < notifyChannels.length; i++) {
			var ch = notifyChannels[i];
			var color = typeBadge[ch.type] || '#666';
			html += '<div class="cfg-row" style="grid-template-columns:1fr auto">' +
				'<div style="display:flex;align-items:center;gap:12px">' +
				'<span style="display:inline-block;padding:2px 10px;border-radius:3px;font-size:11px;font-weight:600;color:#fff;background:' + color + ';min-width:48px;text-align:center">' + (typeNames[ch.type] || ch.type) + '</span>' +
				'<span style="font-weight:600;color:var(--text-primary)">' + escHtml(ch.id) + '</span>' +
				(ch.enabled ? '<span class="dot on" style="margin-left:4px"></span>' : '<span class="dot off" style="margin-left:4px"></span>') +
				'</div>' +
				'<div style="display:flex;gap:8px;align-items:center">' +
				'<button class="cfg-btn cfg-btn-test nch-edit-btn" data-ch-id="' + escAttr(ch.id) + '" style="font-size:11px;padding:4px 10px">编辑</button>' +
				'<button class="cfg-btn cfg-btn-test nch-test-btn" data-ch-id="' + escAttr(ch.id) + '" style="font-size:11px;padding:4px 10px">测试</button>' +
				'<button class="cfg-btn cfg-btn-test nch-delete-btn" data-ch-id="' + escAttr(ch.id) + '" style="font-size:11px;padding:4px 10px;color:var(--danger);border-color:var(--danger)">删除</button>' +
				'</div></div>';
		}
		html += '</div>';
		container.innerHTML = html;
	}

	function showNotifyForm(isEdit) {
		editingChannelId = isEdit || null;
		document.getElementById('notify-form-title').textContent = isEdit ? '编辑渠道: ' + isEdit : '新增渠道';
		document.getElementById('notify-form-panel').style.display = '';

		if (isEdit) {
			document.getElementById('nch-id').disabled = true;
			var ch = findChannel(isEdit);
			if (ch) {
				document.getElementById('nch-id').value = ch.id;
				document.getElementById('nch-type').value = ch.type;
				document.getElementById('nch-url').value = '';
				document.getElementById('nch-secret').value = '';
				document.getElementById('nch-enabled').checked = ch.enabled;
			}
			document.getElementById('btn-nch-test').style.display = '';
		} else {
			document.getElementById('nch-id').disabled = false;
			document.getElementById('nch-id').value = '';
			document.getElementById('nch-type').value = 'dingtalk';
			document.getElementById('nch-url').value = '';
			document.getElementById('nch-secret').value = '';
			document.getElementById('nch-enabled').checked = true;
			document.getElementById('nch-private-ip').checked = false;
			document.getElementById('btn-nch-test').style.display = 'none';
		}
		hideResult('notify-form-result');
	}

	function hideNotifyForm() {
		document.getElementById('notify-form-panel').style.display = 'none';
		editingChannelId = null;
		document.getElementById('nch-id').disabled = false;
		document.getElementById('btn-nch-test').style.display = 'none';
	}

	function findChannel(id) {
		for (var i = 0; i < notifyChannels.length; i++) {
			if (notifyChannels[i].id === id) return notifyChannels[i];
		}
		return null;
	}

	async function saveNotifyChannel() {
		var id = document.getElementById('nch-id').value.trim();
		var type = document.getElementById('nch-type').value;
		var url = document.getElementById('nch-url').value.trim();
		var secret = document.getElementById('nch-secret').value;
		var enabled = document.getElementById('nch-enabled').checked;
		var allowPrivate = document.getElementById('nch-private-ip').checked;

		if (!id) { showResult('notify-form-result', 'fail', '请输入渠道 ID'); return; }
		if (type !== 'log' && !url) { showResult('notify-form-result', 'fail', '请输入 Webhook URL'); return; }

		var body = { id: id, type: type, enabled: enabled, webhook_url: url, allow_private_ip: allowPrivate };
		if (secret) body.secret = secret;

		try {
			var resp = await fetch('/api/notifications/channels', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify(body)
			});
			var data = await resp.json();
			if (data.success) {
				hideNotifyForm();
				loadNotifyChannelsPanel();
				showResult('notify-result', 'ok', '渠道 ' + id + ' 已保存');
			} else {
				showResult('notify-form-result', 'fail', data.error || data.message || '保存失败');
			}
		} catch (e) { showResult('notify-form-result', 'fail', '保存失败: ' + e.message); }
	}

	async function deleteNotifyChannel(id) {
		if (!confirm('确定删除渠道 "' + id + '"？此操作不可撤销。')) return;
		try {
			var resp = await fetch('/api/notifications/channels?id=' + encodeURIComponent(id), {
				method: 'DELETE',
				headers: { 'Authorization': 'Bearer ' + tok() }
			});
			var data = await resp.json();
			if (data.success) {
				if (editingChannelId === id) hideNotifyForm();
				loadNotifyChannelsPanel();
				showResult('notify-result', 'ok', '渠道 ' + id + ' 已删除');
			} else {
				showResult('notify-result', 'fail', data.error || data.message || '删除失败');
			}
		} catch (e) { showResult('notify-result', 'fail', '删除失败: ' + e.message); }
	}

	function editNotifyChannel(id) {
		showNotifyForm(id);
	}

	async function testNotifyChannel(id) {
		var ch = findChannel(id);
		if (!ch) {
			// Use form values if editing a new/unsaved channel
			ch = {
				id: document.getElementById('nch-id').value.trim(),
				type: document.getElementById('nch-type').value,
				webhook_url: document.getElementById('nch-url').value.trim(),
				secret: document.getElementById('nch-secret').value
			};
		}

		// For saved channels, we need the real secret. Prompt if empty.
		var secret = document.getElementById('nch-secret').value || '';
		if (editingChannelId && !secret) {
			// Ask user to re-enter secret for testing (since it's encrypted in storage)
			var input = prompt('请输入签名密钥用于测试（已加密存储，需手动输入）:');
			if (input === null) return;
			secret = input;
		}

		showResult('notify-form-result', 'ok', '测试发送中…');

		try {
			var resp = await fetch('/api/notifications/channels/test', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok() },
				body: JSON.stringify({
					id: ch.id, type: ch.type, webhook_url: ch.webhook_url,
					secret: secret, allow_private_ip: document.getElementById('nch-private-ip').checked
				})
			});
			var data = await resp.json();
			if (data.success) {
				showResult('notify-form-result', 'ok', '测试消息发送成功');
			} else {
				showResult('notify-form-result', 'fail', data.error || data.message || '测试失败');
			}
		} catch (e) { showResult('notify-form-result', 'fail', '测试失败: ' + e.message); }
	}

	function escHtml(s) {
		var d = document.createElement('div');
		d.textContent = s;
		return d.innerHTML;
	}

	function escAttr(s) {
		return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
	}

	// ============================================================
	// Event wiring (runs after all functions are defined)
	// ============================================================

	document.addEventListener('DOMContentLoaded', function() { loadConfig(); });

	// Save buttons (was inline onclick, blocked by CSP)
	var btnEnginesSave = document.getElementById('btn-engines-save');
	if (btnEnginesSave) btnEnginesSave.addEventListener('click', window.saveAllEngines);
	var btnIcpSave = document.getElementById('btn-icp-save');
	if (btnIcpSave) btnIcpSave.addEventListener('click', window.saveICP);
	var btnScreenshotSave = document.getElementById('btn-screenshot-save');
	if (btnScreenshotSave) btnScreenshotSave.addEventListener('click', window.saveScreenshot);
	var btnSystemSave = document.getElementById('btn-system-save');
	if (btnSystemSave) btnSystemSave.addEventListener('click', window.saveSystem);

	// Notification channel buttons
	var btnAdd = document.getElementById('btn-nch-add');
	if (btnAdd) btnAdd.addEventListener('click', function() { showNotifyForm(null); });
	var btnSave = document.getElementById('btn-nch-save');
	if (btnSave) btnSave.addEventListener('click', saveNotifyChannel);
	var btnCancel = document.getElementById('btn-nch-cancel');
	if (btnCancel) btnCancel.addEventListener('click', hideNotifyForm);
	var btnTest = document.getElementById('btn-nch-test');
	if (btnTest) btnTest.addEventListener('click', function() { testNotifyChannel(editingChannelId); });

	// Event delegation for dynamically rendered channel action buttons
	var notifyList = document.getElementById('notify-channel-list');
	if (notifyList) {
		notifyList.addEventListener('click', function(e) {
			var btn = e.target.closest('.nch-edit-btn, .nch-test-btn, .nch-delete-btn');
			if (!btn) return;
			e.preventDefault();
			var id = btn.getAttribute('data-ch-id');
			if (!id) return;
			if (btn.classList.contains('nch-edit-btn')) editNotifyChannel(id);
			else if (btn.classList.contains('nch-test-btn')) testNotifyChannel(id);
			else if (btn.classList.contains('nch-delete-btn')) deleteNotifyChannel(id);
		});
	}

	loadNotifyChannelsPanel();
})();

