// Prepare DayDayMap's stateful SPA search page. The current site does not
// expose a durable query URL: navigation to /searchResult alone loses the
// React model state, so the Bridge must drive the same input/button flow as a
// user before extraction.
export async function prepareDayDayMapSearch(tabId, query, chromeApi = chrome) {
  const initialTab = await chromeApi.tabs.get(tabId);
  const initialURL = new URL(initialTab?.url || "https://www.daydaymap.com/home");
  if (initialURL.pathname.includes("/searchResult") && initialURL.searchParams.get("keyword")) {
    return initialURL.toString();
  }
  const result = await chromeApi.scripting.executeScript({
    target: { tabId },
    func: (nativeQuery) => {
      const inputs = Array.from(document.querySelectorAll("input"));
      const input = inputs.find((element) => {
        const placeholder = (element.getAttribute("placeholder") || "").toLowerCase();
        return placeholder.includes("search") || placeholder.includes("检索") || placeholder.includes("关键词");
      }) || inputs.find((element) => element.type === "text");
      if (!input) return { ok: false, reason: "search_input_missing" };

      const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
      if (descriptor?.set) descriptor.set.call(input, nativeQuery);
      else input.value = nativeQuery;
      input.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: nativeQuery }));
      input.dispatchEvent(new Event("change", { bubbles: true }));

      return { ok: true };
    },
    args: [query]
  });
  const prepared = result?.[0]?.result;
  if (!prepared?.ok) {
    throw new Error(`daydaymap_search_prepare_failed:${prepared?.reason || "unknown"}`);
  }

  // React commits controlled input state asynchronously, so the search click
  // must run in a later task or its handler observes the previous value.
  await new Promise((resolve) => setTimeout(resolve, 600));
  const triggeredResult = await chromeApi.scripting.executeScript({
    target: { tabId },
    func: () => {
      const input = Array.from(document.querySelectorAll("input")).find((element) => {
        const placeholder = (element.getAttribute("placeholder") || "").toLowerCase();
        return placeholder.includes("search") || placeholder.includes("检索") || placeholder.includes("关键词");
      });
      const searchButton = document.querySelector(".search-btn");
      if (searchButton) searchButton.click();
      else if (input?.closest("form")?.requestSubmit) input.closest("form").requestSubmit();
      else if (input) {
        input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", bubbles: true }));
        input.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter", code: "Enter", bubbles: true }));
      } else return { ok: false, reason: "search_trigger_missing" };
      return { ok: true };
    }
  });
  const triggered = triggeredResult?.[0]?.result;
  if (!triggered?.ok) throw new Error(`daydaymap_search_prepare_failed:${triggered?.reason || "unknown"}`);

  await new Promise((resolve) => setTimeout(resolve, 300));
  const current = await chromeApi.tabs.get(tabId);
  if (!(current?.url || "").includes("/searchResult")) {
    await chromeApi.scripting.executeScript({
      target: { tabId },
      func: () => {
        history.pushState({}, "", "/searchResult");
        window.dispatchEvent(new PopStateEvent("popstate"));
      }
    });
  }

  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    const tab = await chromeApi.tabs.get(tabId);
    if ((tab?.url || "").includes("/searchResult")) return tab.url;
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error("daydaymap_search_navigation_timeout");
}
