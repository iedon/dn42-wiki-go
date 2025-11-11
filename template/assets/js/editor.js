const RESERVED_PATHS = new Set([
  "index",
  "_sidebar",
  "_footer",
  "404",
  "_header",
  "layout",
  "readme",
  "search-index",
  "directory",
  "gollum",
  "root",
  "default",
  "assets",
  "api",
]);

export function createEditorModule({ config: runtime, dom, api: apiClient, helpers: util, modal }) {
  if (!runtime.editable) {
    return {
      init() {},
      openEdit() {},
      openNew() {},
      openRename() {},
    };
  }

  const editorModal = dom.qs("#editor-modal");
  const editorTitle = dom.qs("#editor-title");
  const editorInput = dom.qs("#editor-input");
  const editorHighlight = dom.qs("#editor-highlight");
  const editorPreview = dom.qs("#editor-preview");
  const editorTabs = dom.qsa("[data-tab]");
  const editorToolbar = dom.qs("#editor-toolbar");
  const editorWorkspace = dom.qs("#editor-workspace");
  const editorPane = dom.qs("#editor-pane");
  const editorPath = dom.qs("#editor-path");
  const editorPathHint = dom.qs("#editor-path-hint");
  const editorPathGroup = dom.qs("#editor-path-group");
  const editorMessage = dom.qs("#editor-message");
  const editorSave = dom.qs("#editor-save");
  const editorStatus = dom.qs("#editor-status");

  const pathModal = dom.qs("#path-modal");
  const pathInput = dom.qs("#path-input");
  const pathHint = dom.qs("#path-hint");
  const pathStatus = dom.qs("#path-status");
  const pathSubmit = dom.qs("#path-submit");

  let editorInitialContent = "";
  let editorSaving = false;
  let editorMode = "edit";

  function normalizeContent(value) {
    return typeof value === "string" ? value.replace(/\r\n/g, "\n") : "";
  }

  function isReservedPath(path) {
    return RESERVED_PATHS.has((path ?? "").toLowerCase());
  }

  function validateUserPathSyntax(raw) {
    if (typeof raw !== "string") {
      return "";
    }
    const trimmed = raw.trim();
    if (!trimmed) {
      return "";
    }
    if (trimmed.startsWith("/")) {
      return 'Paths must not start with "/"';
    }
    if (trimmed.toLowerCase().endsWith(".md")) {
      return 'Paths must not include the ".md" suffix';
    }
    return "";
  }

  function escapeHTML(value) {
    if (!value) {
      return "";
    }
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function highlightMarkdownSource(raw) {
    if (!raw) {
      return '<span class="hl-placeholder">Start writing...</span>';
    }
    const placeholder = "\uFFF0";
    const blocks = [];
    const tokenised = raw.replace(/```[\s\S]*?```/g, (match) => {
      blocks.push(match);
      return `${placeholder}${blocks.length - 1}${placeholder}`;
    });
    let html = escapeHTML(tokenised);
    html = html.replace(/^#{1,6}\s.*$/gm, (match) => `<span class="hl-heading">${match}</span>`);
    html = html.replace(/^>.*$/gm, (match) => `<span class="hl-quote">${match}</span>`);
    html = html.replace(/(^|\s)(?:-|\*|\+|\d+\.)\s.+$/gm, (match) => `<span class="hl-list">${match}</span>`);
    html = html.replace(/(!?\[[^\]]*\]\([^)]*\))/g, (match) => {
      const cls = match.startsWith("!") ? "hl-image" : "hl-link";
      return `<span class="${cls}">${match}</span>`;
    });
    html = html.replace(/(`[^`]+`)/g, '<span class="hl-code">$1</span>');
    html = html.replace(/(\*\*|__)(?=\S)([\s\S]*?\S)\1/g, '<span class="hl-strong">$1$2$1</span>');
    html = html.replace(/(\*|_)(?=\S)([\s\S]*?\S)\1/g, '<span class="hl-em">$1$2$1</span>');
  html = html.replace(new RegExp(`${placeholder}(\\d+)${placeholder}`, "g"), (_, index) => {
      const block = escapeHTML(blocks[Number(index)] ?? "");
      return `<span class="hl-fence">${block}</span>`;
    });
    if (html.endsWith("\n")) {
      html += "\u200B";
    }
    return html;
  }

  function syncHighlightScroll() {
    if (!editorHighlight || !editorInput) {
      return;
    }
    editorHighlight.scrollTop = editorInput.scrollTop;
    editorHighlight.scrollLeft = editorInput.scrollLeft;
  }

  function updateHighlight() {
    if (!editorHighlight || !editorInput) {
      return;
    }
  editorHighlight.innerHTML = highlightMarkdownSource(editorInput.value || "");
    syncHighlightScroll();
  }

  function setEditorMode(mode) {
    editorMode = mode;
    const showPreview = mode === "preview";
    editorTabs.forEach((button) => {
      const tab = button.getAttribute("data-tab");
      const active = tab === mode;
      button.classList.toggle("active", active);
      button.setAttribute("aria-selected", active ? "true" : "false");
    });
    if (editorPreview) {
      editorPreview.classList.toggle("hidden", !showPreview);
      editorPreview.setAttribute("aria-hidden", showPreview ? "false" : "true");
    }
    if (editorPane) {
      editorPane.classList.toggle("hidden", showPreview);
    }
    if (editorHighlight) {
      editorHighlight.classList.toggle("hidden", showPreview);
    }
    if (editorInput) {
      editorInput.classList.toggle("hidden", showPreview);
      editorInput.setAttribute("aria-hidden", showPreview ? "true" : "false");
      if (!showPreview) {
        updateHighlight();
      }
    }
    if (editorWorkspace) {
      editorWorkspace.classList.toggle("editor-workspace--preview", showPreview);
    }
    if (!showPreview) {
      syncHighlightScroll();
    }
  }

  async function refreshPreview() {
    if (!editorPreview || !editorInput) {
      return;
    }
    editorPreview.innerHTML = "<p>Rendering preview...</p>";
    try {
      const data = await apiClient.fetchJSON("/api/preview", {
        method: "POST",
        body: JSON.stringify({ content: editorInput.value }),
      });
      editorPreview.innerHTML = data.html || "<p>No preview available</p>";
    } catch (error) {
      editorPreview.innerHTML = `<p class="form-hint error">${error.message}</p>`;
    }
  }

  function updateSaveState() {
    if (!editorSave || !editorInput || !editorMessage) {
      return;
    }
    if (editorSaving) {
      editorSave.disabled = true;
      editorSave.setAttribute("aria-disabled", "true");
      return;
    }
    const currentContent = normalizeContent(editorInput.value);
    const hasContent = currentContent.trim().length > 0;
    const hasMessage = editorMessage.value.trim().length > 0;
    const contentChanged = currentContent !== editorInitialContent;
    const shouldEnable = hasContent && hasMessage && contentChanged;
    editorSave.disabled = !shouldEnable;
    editorSave.setAttribute("aria-disabled", editorSave.disabled ? "true" : "false");
  }

  async function savePage() {
    if (!editorInput || !editorPath || !editorMessage || !editorStatus) {
      return;
    }
  const pathValue = editorPath.value.trim() || runtime.pagePath;
  const message = editorMessage.value.trim();
  const currentContent = normalizeContent(editorInput.value);
    if (!pathValue || !message) {
      util.setHint(editorStatus, "Path and commit message are required", true);
      return;
    }
    const mode = editorModal?.dataset?.mode ?? "edit";
    if (mode !== "edit") {
      const syntaxError = validateUserPathSyntax(pathValue);
      if (syntaxError) {
        util.setHint(editorStatus, syntaxError, true);
        return;
      }
    }
    if (currentContent === editorInitialContent) {
      util.setHint(editorStatus, "No content changes to save", true);
      return;
    }
    const storedPath = editorModal?.dataset?.path ?? "";
    const candidateRoute = apiClient.toRoute(pathValue).replace(/^\//, "");
    const reservedCandidate = isReservedPath(candidateRoute);
    const editingSameReserved =
      mode === "edit" &&
      reservedCandidate &&
      storedPath &&
      candidateRoute.toLowerCase() === storedPath.toLowerCase();
    if (reservedCandidate && !editingSameReserved) {
      util.setHint(editorStatus, "The specified path is reserved and cannot be used", true);
      return;
    }
    util.setHint(editorStatus, "Saving...");
    editorSaving = true;
    updateSaveState();
    try {
      await apiClient.fetchJSON("/api/save", {
        method: "POST",
        body: JSON.stringify({
          path: pathValue,
          content: editorInput.value,
          message,
        }),
      });
      util.setHint(editorStatus, "Saved successfully");
      editorInitialContent = currentContent;
      modal.close(editorModal);
      window.location.href = apiClient.pageUrl(pathValue);
    } catch (error) {
      util.setHint(editorStatus, error.message, true);
    } finally {
      editorSaving = false;
      updateSaveState();
    }
  }

  function populateEditor({ path = runtime.pagePath, content = "", trigger = null, isEditing = true }) {
    if (!editorModal || !editorInput || !editorPath) {
      return;
    }
    editorInitialContent = normalizeContent(content);
    if (editorTitle) {
      editorTitle.textContent = isEditing ? "Edit Page" : "New Page";
    }
    const normalizedPath = path ? apiClient.toRoute(path).replace(/^\//, "") : "";
    editorModal.dataset.mode = isEditing ? "edit" : "new";
    editorModal.dataset.path = normalizedPath;
    editorInput.value = content;
    updateHighlight();
    syncHighlightScroll();
    editorPath.value = normalizedPath;
    editorPath.disabled = isEditing;
    if (isEditing) {
      editorPath.setAttribute("aria-disabled", "true");
    } else {
      editorPath.removeAttribute("aria-disabled");
    }
    editorPathGroup?.classList.toggle("hidden", isEditing);
    if (editorPathHint) {
      editorPathHint.textContent = "Paths map to wiki routes. Leave off the .md suffix. No leading slash.";
      editorPathHint.classList.remove("error");
    }
    editorMessage.value = "";
    util.setHint(editorStatus, "");
    editorSaving = false;
    updateSaveState();
    setEditorMode("edit");
    modal.open(editorModal, { trigger });
  }

  async function openEditor(options = {}) {
    populateEditor(options);
  }

  async function handleEdit(trigger) {
    try {
      const params = new URLSearchParams({ path: runtime.pagePath });
      const data = await apiClient.fetchJSON(`/api/document?${params.toString()}`);
      openEditor({
        path: data.path,
        content: data.content || "",
        trigger,
        isEditing: true,
      });
    } catch (error) {
      window.alert(error.message);
    }
  }

  function handleNew(trigger) {
    openEditor({ path: "", content: "", trigger, isEditing: false });
  }

  function handleRename(trigger) {
    if (!pathModal || !pathInput || !pathStatus) {
      return;
    }
    pathInput.value = apiClient.toRoute(runtime.pagePath).replace(/^\//, "");
    if (pathHint) {
      util.setHint(pathHint, "Use folder-style paths without the .md suffix. No leading slash.");
    }
    util.setHint(pathStatus, "");
    if (pathSubmit) {
      pathSubmit.disabled = false;
      pathSubmit.setAttribute("aria-disabled", "false");
    }
    modal.open(pathModal, { trigger });
  }

  async function renamePath() {
    if (!pathInput || !pathStatus) {
      return;
    }
    const newPath = pathInput.value.trim();
    if (!newPath) {
      util.setHint(pathStatus, "Destination path is required", true);
      return;
    }
    const syntaxError = validateUserPathSyntax(newPath);
    if (syntaxError) {
      util.setHint(pathStatus, syntaxError, true);
      return;
    }
    const currentRoute = apiClient.toRoute(runtime.pagePath).replace(/^\//, "");
    const nextRoute = apiClient.toRoute(newPath).replace(/^\//, "");
    if (currentRoute === nextRoute) {
      util.setHint(pathStatus, "Destination path must differ from the current path", true);
      return;
    }
    if (isReservedPath(nextRoute)) {
      util.setHint(pathStatus, "The specified path is reserved and cannot be used", true);
      return;
    }
    util.setHint(pathStatus, "Renaming...");
    if (pathSubmit) {
      pathSubmit.disabled = true;
      pathSubmit.setAttribute("aria-disabled", "true");
    }
    try {
      await apiClient.fetchJSON("/api/rename", {
        method: "POST",
        body: JSON.stringify({
          oldPath: runtime.pagePath,
          newPath,
        }),
      });
      modal.close(pathModal);
      window.location.href = apiClient.pageUrl(newPath);
    } catch (error) {
      util.setHint(pathStatus, error.message, true);
    } finally {
      if (pathSubmit) {
        pathSubmit.disabled = false;
        pathSubmit.setAttribute("aria-disabled", "false");
      }
    }
  }

  function applyFormatting(kind) {
    if (!editorInput) {
      return;
    }
    const textarea = editorInput;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const value = textarea.value;
    const selection = value.slice(start, end);

    function replace(before, after = before) {
      const next = value.slice(0, start) + before + selection + after + value.slice(end);
      textarea.value = next;
      const pos = start + before.length + selection.length + after.length;
      textarea.selectionStart = textarea.selectionEnd = pos;
      textarea.focus();
      updateSaveState();
      updateHighlight();
    }

    function wrapLines(prefix) {
      const before = value.slice(0, start);
      const target = value.slice(start, end);
      const after = value.slice(end);
      const lines = target.split("\n");
      const formatted = lines.map((line) => (line.startsWith(prefix) ? line : `${prefix}${line}`));
      const joined = formatted.join("\n");
      textarea.value = before + joined + after;
      textarea.selectionStart = start;
      textarea.selectionEnd = start + joined.length;
      textarea.focus();
      updateSaveState();
      updateHighlight();
    }

    switch (kind) {
      case "h1":
        wrapLines("# ");
        break;
      case "h2":
        wrapLines("## ");
        break;
      case "h3":
        wrapLines("### ");
        break;
      case "bold":
        replace("**", "**");
        break;
      case "italic":
        replace("*", "*");
        break;
      case "code":
        if (selection.includes("\n")) {
          replace("```\n", "\n```");
        } else {
          replace("`", "`");
        }
        break;
      case "quote":
        wrapLines("> ");
        break;
      case "ul":
        wrapLines("- ");
        break;
      case "ol":
        wrapLines("1. ");
        break;
      case "link":
        replace("[", "](https://)");
        break;
      case "image":
        replace("![", "](https://)");
        break;
      default:
        break;
    }
  }

  function init() {
    if (!editorModal) {
      return;
    }
    if (editorTabs.length) {
      editorTabs.forEach((button) => {
        button.addEventListener("click", () => {
          const tab = button.getAttribute("data-tab");
          if (tab === "preview") {
            refreshPreview();
          }
          setEditorMode(tab);
        });
      });
    }
    editorToolbar?.addEventListener("click", (event) => {
      const target = event.target?.closest("button[data-md]");
      if (!target) {
        return;
      }
      event.preventDefault();
      applyFormatting(target.getAttribute("data-md"));
    });
    if (editorInput) {
      editorInput.addEventListener("input", () => {
        updateSaveState();
        updateHighlight();
      });
      editorInput.addEventListener("scroll", syncHighlightScroll);
    }
    editorMessage?.addEventListener("input", updateSaveState);
    editorSave?.addEventListener("click", (event) => {
      event.preventDefault();
      savePage();
    });
    pathSubmit?.addEventListener("click", (event) => {
      event.preventDefault();
      renamePath();
    });
  }

  return {
    init,
    openEdit: handleEdit,
    openNew: handleNew,
    openRename: handleRename,
  };
}
