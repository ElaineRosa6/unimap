import { apiGet, apiPostBridgeSigned, bridgeRotateToken } from "./api.js";
import { ensureTab, waitForPageReady, captureVisible, normalizeImagePayload, releaseTab, cleanupTabPool, normalizeCollectPayload, extractEngineAssets } from "./capture.js";
import { loadSessionToken, isTokenExpired, saveSessionToken, saveRuntimeState, saveLastError } from "./storage.js";
import { pairAndStore } from "./pairing.js";

const POLL_INTERVAL_MS = 1000;
const CAPTURE_MIN_INTERVAL_MS = 1200;
const ROTATE_AHEAD_MS = 60 * 1000;
let loopStarted = false;
let lastCaptureAt = 0;

function shouldRotateSoon(expireAt) {
  if (!expireAt) {
    return false;
  }
  return Date.now() + ROTATE_AHEAD_MS >= expireAt;
}

function isBridgeAuthError(err) {
  const text = String(err || "").toLowerCase();
  return text.includes("unauthorized_bridge") || text.includes("401");
}

async function pollTaskOnce(token) {
  const resp = await apiGet("/api/screenshot/bridge/tasks/next", token);
  return resp?.task || null;
}

async function reportTaskResult(result, token) {
  await apiPostBridgeSigned("/api/screenshot/bridge/mock/result", result, token);
}

async function waitForCaptureSlot() {
  const elapsed = Date.now() - lastCaptureAt;
  if (elapsed < CAPTURE_MIN_INTERVAL_MS) {
    await new Promise((resolve) => setTimeout(resolve, CAPTURE_MIN_INTERVAL_MS - elapsed));
  }
  lastCaptureAt = Date.now();
}

async function handleTask(task, token) {
  const startedAt = Date.now();
  const requestId = task.request_id;
  const action = task.action || "screenshot";
  let tabId = null;

  async function captureWithFocus(tid, windowId) {
    await waitForCaptureSlot();
    await chrome.tabs.update(tid, { active: true });
    await chrome.windows.update(windowId, { focused: true });
    await new Promise((resolve) => setTimeout(resolve, 300));
    return captureVisible();
  }

  async function reportResult(result) {
    result.batch_id = task.batch_id || "";
    result.url = task.url || "";
    await reportTaskResult(result, token);
  }

  async function reportFailure(err) {
    const durationMs = Math.max(1, Date.now() - startedAt);
    const errorText = String(err || "plugin_task_failed");
    await reportResult({
      request_id: requestId,
      success: false,
      image_path: "",
      image_data: "",
      duration_ms: durationMs,
      error_code: "plugin_task_failed",
      error: errorText
    });
  }

  try {
    // Cookie-read tasks don't need a tab — read directly via chrome.cookies API.
    if (action === "get_cookies") {
      const cookies = await chrome.cookies.getAll({ domain: task.url });
      const durationMs = Math.max(1, Date.now() - startedAt);
      await reportResult({
        request_id: requestId,
        success: true,
        image_path: "",
        image_data: "",
        collected_data: JSON.stringify(cookies),
        structured_collected_data: { cookies: cookies },
        duration_ms: durationMs
      });
      return;
    }

    tabId = await ensureTab(task.url);

    // Choose wait strategy based on action type
    let waitStrategy = task.wait_strategy || "load";
    if (action === "collect") {
      // Collect needs extra time for SPA rendering
      waitStrategy = "spa";
    }

    await waitForPageReady(tabId, waitStrategy, task.timeout_ms || 15000);

    if (action === "open") {
      // Only open the page, no screenshot or data collection
      const durationMs = Math.max(1, Date.now() - startedAt);
      await reportResult({
        request_id: requestId,
        success: true,
        image_path: "",
        image_data: "",
        duration_ms: durationMs
      });
    } else if (action === "collect") {
      // Extract structured DOM data from the page
      const assets = await extractEngineAssets(tabId);

      // Handle login wall detection
      if (assets.is_login_wall) {
        const durationMs = Math.max(1, Date.now() - startedAt);
        await reportResult({
          request_id: requestId,
          success: false,
          image_path: "",
          image_data: "",
          duration_ms: durationMs,
          error_code: "login_required",
          error: `login wall detected on ${assets.engine || "unknown"}`,
          collected_data: "",
          structured_collected_data: {
            title: assets.title || "",
            items: [],
            total: 0,
            has_more: false,
            is_login_wall: true,
            engine: assets.engine
          }
        });
      } else {
        const result = normalizeCollectPayload(
          assets.items,
          assets.title,
          requestId,
          startedAt
        );

        // Override total and has_more from extraction result
        if (result.structured_collected_data) {
          result.structured_collected_data.total = assets.total || assets.items.length;
          result.structured_collected_data.has_more = assets.has_more || false;
          result.structured_collected_data.engine = assets.engine || "unknown";
          if (assets.error) {
            result.structured_collected_data.extraction_error = assets.error;
          }
        }

        // Include raw error info if extraction failed but didn't login wall
        if (assets.error && assets.items.length === 0) {
          result.structured_collected_data.extraction_error = assets.error;
        }

        await reportResult(result);
      }
    } else {
      // "screenshot" or unknown action — capture screenshot
      await waitForCaptureSlot();
      const tab = await chrome.tabs.get(tabId);
      let dataUrl;
      try {
        dataUrl = await captureWithFocus(tabId, tab.windowId);
      } catch (captureErr) {
        await waitForCaptureSlot();
        dataUrl = await captureWithFocus(tabId, tab.windowId);
      }
      const result = normalizeImagePayload(dataUrl, requestId, startedAt);
      await reportResult(result);
    }

    await saveRuntimeState({
      last_task_id: requestId,
      last_success_at: Date.now()
    });

    // Release tab after successful task
    await releaseTab(tabId);
    tabId = null;
  } catch (err) {
    await reportFailure(err);

    // Release tab after error
    if (tabId) {
      await releaseTab(tabId);
      tabId = null;
    }
    throw err;
  }
}

async function bridgeLoop() {
  if (loopStarted) {
    return;
  }
  loopStarted = true;

  for (;;) {
    try {
      // Periodically clean up stale tabs in the pool
      await cleanupTabPool();

      const session = await loadSessionToken();
      let token = session.token;
      if (!token || isTokenExpired(session.expireAt)) {
        try {
          const pair = await pairAndStore(chrome.runtime.id, "dev-pair");
          token = pair.token;
        } catch (pairErr) {
          await saveRuntimeState({ paired: false });
          await saveLastError(pairErr);
          await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
          continue;
        }
      } else if (shouldRotateSoon(session.expireAt)) {
        try {
          const rotated = await bridgeRotateToken(token);
          const newToken = rotated?.token || "";
          const expiresIn = Number(rotated?.expires_in || 600);
          if (newToken) {
            await saveSessionToken(newToken, expiresIn);
            token = newToken;
          }
        } catch (rotateErr) {
          // Rotation failure should not stop task polling; existing token may still be valid.
          await saveLastError(rotateErr);
        }
      }

      await saveRuntimeState({ paired: true });
      const task = await pollTaskOnce(token);
      if (task && task.request_id && task.url) {
        await handleTask(task, token);
      }
    } catch (err) {
      if (isBridgeAuthError(err)) {
        await saveSessionToken("", 1);
        await saveRuntimeState({ paired: false });
      }
      await saveLastError(err);
    }

    await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
  }
}

chrome.runtime.onInstalled.addListener(() => {
  bridgeLoop();
});

chrome.runtime.onStartup.addListener(() => {
  bridgeLoop();
});

bridgeLoop();
