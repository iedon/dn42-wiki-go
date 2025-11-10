export function createSearchModule({
  config: runtime,
  dom,
  api: apiClient,
  helpers: util,
}) {
  const input = dom.qs("#search-box");
  const results = dom.qs("#search-results");
  const container = dom.qs("[data-search-container]");
  const initialPlaceholder = input?.placeholder ?? "";
  const state = {
    indexPromise: null,
    indexData: null,
    activeIndex: -1,
  };
  const digitRegex = /^[0-9]$/;

  function normalizeIndexPath(candidate) {
    if (!candidate) {
      return "";
    }
    const value = candidate.trim();
    if (!value) {
      return "";
    }
    if (/^https?:\/\//i.test(value)) {
      return value;
    }
    if (value.includes("?") || value.startsWith("/api/")) {
      return "";
    }
    if (value.startsWith("/")) {
      return value.replace(/\/{2,}/g, "/");
    }
    const prefix = runtime.basePath
      ? `${runtime.basePath}/${value}`
      : `/${value}`;
    return prefix.replace(/\/{2,}/g, "/");
  }

  function resolveIndexPath() {
    const preferred = normalizeIndexPath(runtime.searchIndexPath);
    if (preferred) {
      return preferred;
    }
    const legacy = normalizeIndexPath(runtime.legacySearchPath);
    if (legacy) {
      return legacy;
    }
    const fallback = runtime.basePath
      ? `${runtime.basePath}/search-index.json`
      : "/search-index.json";
    return fallback;
  }

  function disableSearch(message = "Search unavailable") {
    container?.removeAttribute("hidden");
    if (input) {
      input.disabled = false;
      input.removeAttribute("aria-disabled");
      input.dataset.placeholderOverride = "true";
      input.placeholder = message;
    }
    showMessage(message, "search-error");
  }

  function restorePlaceholder() {
    if (input && input.dataset.placeholderOverride) {
      input.placeholder = initialPlaceholder;
      delete input.dataset.placeholderOverride;
    }
  }

  function clearResults() {
    if (!results) {
      return;
    }
    results.innerHTML = "";
    results.classList.add("hidden");
    state.activeIndex = -1;
  }

  function createMessage(text, className) {
    const div = document.createElement("div");
    div.className = className;
    div.textContent = text;
    return div;
  }

  function showMessage(text, className) {
    if (!results) {
      return;
    }
    results.replaceChildren(createMessage(text, className));
    results.classList.remove("hidden");
    state.activeIndex = -1;
  }

  function renderResults(items) {
    if (!results) {
      return;
    }
    if (!items.length) {
      showMessage("No match found", "search-message");
      return;
    }
    const fragment = document.createDocumentFragment();
    items.forEach((item, index) => {
      fragment.appendChild(createResultItem(item, index));
    });
    results.replaceChildren(fragment);
    results.classList.remove("hidden");
    state.activeIndex = -1;
  }

  function createResultItem(item, index) {
    const div = document.createElement("div");
    div.className = "search-hit";
    div.setAttribute("tabindex", "0");
    div.dataset.index = String(index);
    div.dataset.href = item.route;
    div.innerHTML = `<strong>${item.title}</strong><span>${item.snippet}</span>`;
    return div;
  }

  function setActiveResult(index) {
    if (!results) {
      return;
    }
    const items = dom.qsa(".search-hit", results);
    items.forEach((item, idx) => {
      item.classList.toggle("active", idx === index);
    });
    state.activeIndex = index;
  }

  function selectActiveResult() {
    if (!results) {
      return;
    }
    const items = dom.qsa(".search-hit", results);
    if (!items.length) {
      return;
    }
    let target = null;
    if (state.activeIndex >= 0 && state.activeIndex < items.length) {
      target = items[state.activeIndex];
    } else {
      target = items[0];
    }
    if (!target) {
      return;
    }
    const href = target.dataset.href;
    if (href) {
      window.location.assign(apiClient.pageUrl(href));
    }
  }

  function decodeInt(value) {
    if (value == null) {
      return 0;
    }
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
    const digits = "0123456789abcdefghijklmnopqrstuvwxyz";
    const normalized = String(value).trim().toLowerCase();
    if (!normalized) {
      return 0;
    }
    let result = 0;
    for (const ch of normalized) {
      const idx = digits.indexOf(ch);
      if (idx < 0) {
        return 0;
      }
      result = result * 36 + idx;
    }
    return result;
  }

  function decodePositions(encoded) {
    if (!encoded) {
      return [];
    }
    const parts = encoded.split(".");
    const positions = [];
    let total = 0;
    parts.forEach((part) => {
      const value = decodeInt(part);
      total += value;
      positions.push(total);
    });
    return positions;
  }

  function decodePosting(entry) {
    const [docId, titleFreq, summaryFreq, contentFreq, encodedPositions] =
      entry.split(":");
    return {
      docId: decodeInt(docId),
      titleFreq: decodeInt(titleFreq),
      summaryFreq: decodeInt(summaryFreq),
      contentFreq: decodeInt(contentFreq),
      positions: decodePositions(encodedPositions || ""),
    };
  }

  function decodeLengths(meta) {
    if (!meta) {
      return [0, 0, 0];
    }
    return meta.split(",").map((value) => decodeInt(value));
  }

  function decodeTerm(encoded) {
    if (!encoded) {
      return [];
    }
    const parts = encoded.split(";");
    const [header, ...rest] = parts;
    const expected = decodeInt(header.split(":")[0]);
    const postings = rest.map(decodePosting);
    if (expected && postings.length !== expected) {
      return postings.slice(0, expected);
    }
    return postings;
  }

  function shouldKeepToken(token) {
    if (!token) {
      return false;
    }
    if (token.length === 1) {
      return digitRegex.test(token);
    }
    return true;
  }

  function tokenize(text) {
    if (!text) {
      return [];
    }
    const normalized = window.unorm
      ? window.unorm.nfkd(text)
      : text.normalize("NFKD");
    const tokens = [];
    let buffer = "";
    for (const ch of normalized) {
      if (/^[\p{L}\p{N}]$/u.test(ch)) {
        buffer += ch.toLowerCase();
      } else if (buffer) {
        if (shouldKeepToken(buffer)) {
          tokens.push(buffer);
        }
        buffer = "";
      }
    }
    if (buffer && shouldKeepToken(buffer)) {
      tokens.push(buffer);
    }
    return tokens;
  }

  function prepareIndex(payload) {
    if (!payload) {
      return null;
    }
    const rawDocs = Array.isArray(payload.docs)
      ? payload.docs
      : Array.isArray(payload.d)
      ? payload.d
      : null;
    if (!rawDocs) {
      return null;
    }
    const docs = rawDocs.map(([route, title, summary, meta]) => ({
      route,
      title,
      summary,
      lengths: decodeLengths(meta),
    }));
    const termSource = payload.terms ?? payload.t ?? {};
    const terms = new Map();
    Object.keys(termSource).forEach((term) => {
      terms.set(term, decodeTerm(termSource[term]));
    });
    const fieldList = payload.fields ??
      payload.f ?? ["title", "summary", "content"];
    const avgLengthsRaw = payload.avgFieldLengths ?? payload.a ?? [];
    const docCount = payload.docCount ?? payload.c ?? docs.length;
    return {
      version: payload.v ?? payload.version ?? 0,
      docCount,
      fields: fieldList,
      avgFieldLengths: avgLengthsRaw.map((value) => decodeInt(value)),
      docs,
      terms,
    };
  }

  function computeIDF(df, docCount) {
    if (!df || !docCount) {
      return 0;
    }
    return Math.log((docCount - df + 0.5) / (df + 0.5) + 1);
  }

  function bm25(tf, fieldLength, avgFieldLength) {
    if (!tf) {
      return 0;
    }
    const k1 = 1.5;
    const b = 0.75;
    const norm = 1 - b + (b * fieldLength) / (avgFieldLength || 1);
    return (tf * (k1 + 1)) / (tf + k1 * norm);
  }

  function computePhraseBonus(matches, termCount) {
    if (!matches || !termCount) {
      return 0;
    }
    const sequences = new Map();
    matches.forEach((position) => {
      sequences.set(position, (sequences.get(position) || 0) + 1);
    });
    const bonus = Array.from(sequences.values()).reduce((acc, count) => {
      return count >= termCount ? acc + termCount * 1.5 : acc + count * 0.5;
    }, 0);
    return bonus;
  }

  function computeTitleBoost(title, tokens) {
    if (!title || !tokens.length) {
      return 0;
    }
    const lower = title.toLowerCase();
    return tokens.reduce(
      (acc, token) => (lower.includes(token) ? acc + 1 : acc),
      0
    );
  }

  function computePathBoost(route, tokens) {
    if (!route || !tokens.length) {
      return 0;
    }
    const lower = route.toLowerCase();
    return tokens.reduce(
      (acc, token) => (lower.includes(token) ? acc + 0.5 : acc),
      0
    );
  }

  function buildSnippet(doc, tokens) {
    const summary = doc.summary ?? "";
    if (!summary) {
      return "";
    }
    const lower = summary.toLowerCase();
    const positions = tokens
      .map((token) => lower.indexOf(token))
      .filter((pos) => pos >= 0)
      .sort((a, b) => a - b);
    if (!positions.length) {
      return summary.slice(0, 140);
    }
    const start = Math.max(positions[0] - 40, 0);
    const end = Math.min(start + 160, summary.length);
    return `${summary.slice(start, end)}${end < summary.length ? "..." : ""}`;
  }

  function expandTokenByPrefix(token, index) {
    if (!token || !index) {
      return [];
    }
    const matches = [];
    index.terms.forEach((value, key) => {
      if (key.startsWith(token)) {
        matches.push(key);
      }
    });
    return matches.length ? matches : [token];
  }

  function collectTermEntry(token, index) {
    if (!token) {
      return [];
    }
    const expanded = expandTokenByPrefix(token, index);
    const combined = new Map();
    expanded.forEach((expansion) => {
      const postings = index.terms.get(expansion) ?? [];
      postings.forEach((posting) => {
        const bucket = combined.get(posting.docId) || {
          docId: posting.docId,
          title: 0,
          summary: 0,
          content: 0,
          positions: [],
        };
        bucket.title += posting.titleFreq;
        bucket.summary += posting.summaryFreq;
        bucket.content += posting.contentFreq;
        bucket.positions.push(...posting.positions);
        combined.set(posting.docId, bucket);
      });
    });
    return Array.from(combined.values());
  }

  function searchDocuments(index, query) {
    if (!index) {
      return [];
    }
    const tokens = tokenize(query);
    if (!tokens.length) {
      return [];
    }
    const postings = tokens.map((token) => collectTermEntry(token, index));
    if (!postings.every((item) => item.length)) {
      return [];
    }
    const docScores = new Map();
    tokens.forEach((token, tokenIndex) => {
      const entries = postings[tokenIndex];
      entries.forEach((entry) => {
        const doc = index.docs[entry.docId];
        if (!doc) {
          return;
        }
        const key = entry.docId;
        const prev = docScores.get(key) || { score: 0, matches: [] };
        const idf = computeIDF(entries.length, index.docCount);
        const [titleLen, summaryLen, contentLen] = doc.lengths;
        const score =
          prev.score +
          idf *
            (bm25(entry.title, titleLen, index.avgFieldLengths[0]) * 2 +
              bm25(entry.summary, summaryLen, index.avgFieldLengths[1]) +
              bm25(entry.content, contentLen, index.avgFieldLengths[2]));
        prev.matches.push(...entry.positions);
        docScores.set(key, { score, matches: prev.matches });
      });
    });
    const scores = Array.from(docScores.entries()).map(([docId, info]) => {
      const doc = index.docs[docId];
      const titleBonus = computeTitleBoost(doc.title, tokens);
      const pathBonus = computePathBoost(doc.route, tokens);
      const phraseBonus = computePhraseBonus(info.matches, tokens.length);
      return {
        docId,
        score: info.score + titleBonus * 1.5 + pathBonus + phraseBonus,
        snippet: buildSnippet(doc, tokens),
      };
    });
    scores.sort((a, b) => b.score - a.score);
    return scores.slice(0, 10).map((item) => {
      const doc = index.docs[item.docId];
      return {
        route: doc.route,
        title: doc.title,
        snippet: item.snippet,
      };
    });
  }

  function handleKeyDown(event) {
    if (!results || results.classList.contains("hidden")) {
      return;
    }
    const items = dom.qsa(".search-hit", results);
    if (!items.length) {
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      const next = (state.activeIndex + 1) % items.length;
      setActiveResult(next);
      items[next].focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      const next = (state.activeIndex - 1 + items.length) % items.length;
      setActiveResult(next);
      items[next].focus();
    } else if (event.key === "Enter") {
      event.preventDefault();
      selectActiveResult();
    } else if (event.key === "Escape") {
      clearResults();
    }
  }

  function registerResultHandlers() {
    if (!results) {
      return;
    }
    results.addEventListener("click", (event) => {
      const item = event.target?.closest(".search-hit");
      if (!item) {
        return;
      }
      event.preventDefault();
      const href = item.dataset.href;
      if (href) {
        window.location.assign(apiClient.pageUrl(href));
      }
    });
    results.addEventListener("mouseover", (event) => {
      const item = event.target?.closest(".search-hit");
      if (!item) {
        return;
      }
      const index = Number(item.dataset.index ?? "-1");
      if (!Number.isNaN(index)) {
        setActiveResult(index);
      }
    });
  }

  async function ensureIndexLoaded() {
    if (state.indexData) {
      return state.indexData;
    }
    if (!state.indexPromise) {
      const resolvedPath = resolveIndexPath();
      if (!resolvedPath) {
        throw new Error("Missing search index path");
      }
      state.indexPromise = apiClient
        .fetchJSON(resolvedPath)
        .then((payload) => {
          state.indexData = prepareIndex(payload);
          if (!state.indexData) {
            throw new Error("Search index unavailable");
          }
          return state.indexData;
        })
        .catch((error) => {
          state.indexPromise = null;
          throw error;
        });
    }
    return state.indexPromise;
  }

  async function handleInput(event) {
    restorePlaceholder();
    const query = event.target.value.trim();
    if (!query) {
      clearResults();
      return;
    }
    try {
      const index = await ensureIndexLoaded();
      const items = searchDocuments(index, query);
      renderResults(items);
    } catch (error) {
      disableSearch(error.message);
    }
  }

  function init() {
    if (!runtime.searchEnabled || !input || !results) {
      return;
    }
    registerResultHandlers();
    input.addEventListener("input", util.debounce(handleInput, 120));
    input.addEventListener("keydown", handleKeyDown);
    clearResults();
  }

  return { init };
}
