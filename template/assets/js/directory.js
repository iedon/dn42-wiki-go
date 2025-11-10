export function createDirectoryModule({ config: runtime, dom, helpers: util }) {
  const ACTIVE_CLASS = "directory-item--active";

  function isDirectoryPage() {
    return runtime.pagePath === "/directory" || runtime.pagePath === "/directory.html";
  }

  function parseHash() {
    const raw = window.location.hash.replace(/^#/, "");
    if (!raw) {
      return "";
    }
    try {
      return decodeURIComponent(raw).trim();
    } catch (_error) {
      return raw.trim();
    }
  }

  function findTarget(hash) {
    if (!hash) {
      return null;
    }
    const candidates = [hash];
    const lowered = hash.toLowerCase();
    if (lowered !== hash) {
      candidates.push(lowered);
    }

    for (const value of candidates) {
      const direct = document.getElementById(value);
      if (direct) {
        return direct;
      }
    }

    for (const value of candidates) {
      const escaped = util.escapeSelector(value);
      if (!escaped) {
        continue;
      }
      const match =
        document.querySelector(`[data-directory-anchor="${escaped}"]`) ||
        document.querySelector(`[data-directory-aliases~="${escaped}"]`);
      if (match) {
        return match;
      }
    }

    return null;
  }

  function openAncestors(element) {
    let current = element ? element.parentElement : null;
    while (current) {
      if (current.tagName === "DETAILS") {
        current.open = true;
      }
      current = current.parentElement;
    }
  }

  let activeItem = null;

  function setActive(item) {
    if (!item) {
      return;
    }
    if (activeItem && activeItem !== item) {
      activeItem.classList.remove(ACTIVE_CLASS);
    }
    activeItem = item;
    item.classList.add(ACTIVE_CLASS);
  }

  function revealHash(hash, { smooth = true } = {}) {
    if (!isDirectoryPage()) {
      return;
    }
    const target = findTarget(hash);
    if (!(target instanceof Element)) {
      return;
    }
    const hasClassList =
      target.classList && typeof target.classList.contains === "function";
    const targetIsItem = hasClassList && target.classList.contains("directory-item");
    const item = targetIsItem ? target : target.closest(".directory-item");

    if (!item) {
      return;
    }

    openAncestors(item);
    setActive(item);

    window.requestAnimationFrame(() => {
      item.scrollIntoView({
        block: Number(item.dataset.depth || 0) > 2 ? "center" : "start",
        behavior: smooth ? "smooth" : "auto",
      });
    });
  }

  function handleHashChange() {
    revealHash(parseHash(), { smooth: true });
  }

  function handleDirectoryClick(event) {
    if (!isDirectoryPage()) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
  const summary = target.closest(".directory-branch > summary");
  const link = summary ? null : target.closest(".directory-item a");
    const item = (summary || link)?.closest?.(".directory-item");
    if (!item) {
      return;
    }
    setActive(item);
    openAncestors(item);
    const anchor = item.getAttribute("data-directory-anchor");
    if (summary && anchor && typeof history !== "undefined" && typeof history.replaceState === "function") {
      const currentHash = window.location.hash.replace(/^#/, "");
      if (currentHash !== anchor) {
        history.replaceState(null, "", `#${anchor}`);
      }
    }
  }

  function init() {
    if (!isDirectoryPage()) {
      return;
    }

    const directory = dom.qs(".directory");
    if (!directory) {
      return;
    }

    revealHash(parseHash(), { smooth: false });
    window.addEventListener("hashchange", handleHashChange);
    directory.addEventListener("click", handleDirectoryClick);
  }

  return { init };
}
