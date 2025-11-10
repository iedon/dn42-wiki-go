export function createHistoryModule({ config: runtime, dom, api: apiClient, helpers: util, modal }) {
  if (!runtime.editable) {
    return { init() {}, open() {} };
  }

  const historyModal = dom.qs("#history-modal");
  const historyList = dom.qs("#history-list");
  const historyLoadMore = dom.qs("#history-load-more");
  const historyViewDiff = dom.qs("#history-view-diff");
  const historyDiffModal = dom.qs("#history-diff-modal");
  const historyDiff = dom.qs("#history-diff");
  const historyDiffCode = dom.qs("#history-diff-code");
  const historyDiffTitle = dom.qs("#history-diff-title");

  const state = {
    page: 0,
    count: 0,
    hasMore: false,
    loading: false,
    selected: [],
  };

  function updateHistoryLoadMoreButton() {
    if (!historyLoadMore) {
      return;
    }
    historyLoadMore.disabled = !state.hasMore || state.loading;
    historyLoadMore.setAttribute("aria-disabled", historyLoadMore.disabled ? "true" : "false");
    historyLoadMore.textContent = state.loading ? "Loading..." : "Load more";
  }

  function resetHistoryDiff(forceClose = false, preserveSelection = false) {
    if (!preserveSelection) {
      state.selected = [];
    }
    if (historyViewDiff) {
      historyViewDiff.disabled = true;
      historyViewDiff.setAttribute("aria-disabled", "true");
    }
    if (historyDiff && historyDiffCode) {
      historyDiffCode.textContent = "";
    }
    if (forceClose && historyDiffModal) {
      modal.close(historyDiffModal);
    }
  }

  function setHistoryDiffTitle(fromHash, toHash) {
    if (historyDiffTitle) {
      historyDiffTitle.textContent = `Diff ${fromHash.slice(0, 12)} \u2794 ${toHash.slice(0, 12)}`;
    }
  }

  function showDiffStatus(message, toneClass = "") {
    if (!historyDiffCode) {
      return;
    }
    historyDiffCode.textContent = message;
    historyDiffCode.className = toneClass || "";
  }

  function renderDiffText(diffText) {
    if (!historyDiffCode) {
      return;
    }
    const lines = diffText.split("\n");
    const fragment = document.createDocumentFragment();
    lines.forEach((line) => {
      const span = document.createElement("span");
      span.textContent = line;
      if (line.startsWith("+")) {
        span.classList.add("line-add");
      } else if (line.startsWith("-")) {
        span.classList.add("line-del");
      } else if (line.startsWith("@@")) {
        span.classList.add("line-info");
      } else if (line.startsWith("diff")) {
        span.classList.add("line-meta");
      }
      fragment.appendChild(span);
      fragment.appendChild(document.createTextNode("\n"));
    });
    historyDiffCode.innerHTML = "";
    historyDiffCode.appendChild(fragment);
  }

  function renderHistoryItems(items, offset, reset) {
    if (!historyList) {
      return;
    }
    if (reset) {
      historyList.innerHTML = "";
    }
    if (!items.length && !offset) {
      const empty = document.createElement("div");
      empty.className = "history-empty";
      empty.textContent = "No history entries";
      historyList.appendChild(empty);
      return;
    }
    items.forEach((item, index) => {
      const container = document.createElement("div");
      container.className = "history-entry";
      const labelId = `history-${offset + index}`;
      container.innerHTML = `
        <div class="history-title">${item.message || "No commit message"}</div>
        <div class="history-meta">
          <span>${util.formatDate(item.committedAt)}</span>
          <span>${item.author || "Unknown"}${item.email ? ` &lt;${item.email}&gt;` : ''}</span>
          <code>${item.hash.slice(0, 12)}</code>
        </div>
        <div class="history-actions">
          <input type="checkbox" id="${labelId}" data-index="${offset + index}" value="${item.hash}">
          <label for="${labelId}" class="history-select">Compare</label>
        </div>        
      `;
      const checkbox = container.querySelector('input[type="checkbox"]');
      checkbox?.addEventListener("change", () => toggleHistorySelection(checkbox));
      historyList.appendChild(container);
    });
  }

  function toggleHistorySelection(checkbox) {
    if (!checkbox) {
      return;
    }
    const hash = checkbox.value;
    const index = Number(checkbox.dataset.index ?? "0");
    if (checkbox.checked) {
      state.selected.push({ hash, index });
      if (state.selected.length > 2) {
        const removed = state.selected.shift();
        const other = historyList?.querySelector(`input[type="checkbox"][value="${removed.hash}"]`);
        if (other) {
          other.checked = false;
        }
      }
    } else {
      const pos = state.selected.findIndex((entry) => entry.hash === hash);
      if (pos >= 0) {
        state.selected.splice(pos, 1);
      }
    }
    const enabled = state.selected.length === 2;
    if (historyViewDiff) {
      historyViewDiff.disabled = !enabled;
      historyViewDiff.setAttribute("aria-disabled", enabled ? "false" : "true");
    }
    if (!enabled) {
      resetHistoryDiff(true, true);
    }
  }

  async function loadHistory(reset = false) {
    if (!historyModal || state.loading) {
      return;
    }
    state.loading = true;
    updateHistoryLoadMoreButton();
    const page = reset ? 0 : state.page;
    const offset = reset ? 0 : state.count;
    const params = new URLSearchParams({
      path: runtime.pagePath,
      page: String(page),
      pageSize: "25",
    });
    try {
      const data = await apiClient.fetchJSON(`/api/history?${params.toString()}`);
      const items = data.items ?? [];
      if (reset) {
        resetHistoryDiff(true);
        state.count = 0;
      }
      renderHistoryItems(items, offset, reset);
      state.count = offset + items.length;
      state.hasMore = Boolean(data.hasMore);
      state.page = page + 1;
    } catch (error) {
      if (historyList) {
        historyList.innerHTML = "";
        const div = document.createElement("div");
        div.className = "history-error";
        div.textContent = error.message || "Unable to load history";
        historyList.appendChild(div);
      }
      state.hasMore = false;
    } finally {
      state.loading = false;
      updateHistoryLoadMoreButton();
    }
  }

  async function showDiff() {
    if (state.selected.length !== 2 || !historyDiffModal || !historyDiff) {
      return;
    }
    const [from, to] = state.selected.slice().sort((a, b) => a.index - b.index);
    setHistoryDiffTitle(from.hash, to.hash);
    modal.open(historyDiffModal, { stack: true, trigger: historyViewDiff });
    showDiffStatus("Loading diff...", "z-go");
    try {
      const params = new URLSearchParams({
        path: runtime.pagePath,
        from: from.hash,
        to: to.hash,
      });
      const data = await apiClient.fetchJSON(`/api/diff?${params.toString()}`);
      const diff = (data && data.diff) || "";
      if (!diff.trim()) {
        showDiffStatus("No diff available", "z-go");
      } else {
        renderDiffText(diff);
      }
    } catch (error) {
      showDiffStatus(error.message || "Unable to load diff", "z-gd");
    }
  }

  function open(trigger) {
    if (!historyModal) {
      return;
    }
    if (historyList) {
      historyList.innerHTML = '<div class="history-loading">Loading...</div>';
    }
    state.page = 0;
    state.count = 0;
    state.hasMore = false;
    resetHistoryDiff(true);
    updateHistoryLoadMoreButton();
    modal.open(historyModal, { trigger });
    loadHistory(true);
  }

  function init() {
    if (!historyModal) {
      return;
    }
    historyLoadMore?.addEventListener("click", (event) => {
      event.preventDefault();
      loadHistory(false);
    });
    historyViewDiff?.addEventListener("click", (event) => {
      event.preventDefault();
      showDiff();
    });
  }

  return { init, open };
}
