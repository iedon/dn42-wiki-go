export function createDirectoryModule({ config: runtime, dom, helpers: util }) {
  function prepareDirectoryPageAnchors() {
    const pageRoute = runtime.pagePath;
    if (pageRoute !== "/directory" && pageRoute !== "/directory.html") {
      return;
    }
    const container = dom.qs(".content");
    if (!container) {
      return;
    }
    const rootList = container.querySelector("ul");
    if (!rootList) {
      return;
    }
    Array.from(rootList.children).forEach((child) => {
      if (!child || child.tagName !== "LI" || child.parentElement !== rootList || child.id) {
        return;
      }
      let slug = "";
      const firstLink = child.querySelector("a[href]");
      if (firstLink) {
        slug = firstRouteSegmentFromHref(firstLink.getAttribute("href"));
      }
      if (!slug) {
        const label = child.querySelector("p");
        slug = util.slugify(label ? label.textContent : child.textContent);
      }
      if (!slug) {
        return;
      }
      child.id = slug;
      child.dataset.anchor = slug;
    });
  }

  function firstRouteSegmentFromHref(href) {
    if (!href) {
      return "";
    }
    let urlPath = href;
    if (/^https?:/i.test(href)) {
      try {
        const url = new URL(href, window.location.origin);
        urlPath = url.pathname;
      } catch (_error) {
        return "";
      }
    }
    const pathOnly = urlPath.split("#")[0].split("?")[0];
    if (!pathOnly.startsWith("/")) {
      return "";
    }
    const segment = pathOnly.replace(/^\/+/, "").split("/")[0];
    return util.slugify(segment);
  }

  function scrollToDirectoryPageHash() {
    const pageRoute = runtime.pagePath;
    if (pageRoute !== "/directory" && pageRoute !== "/directory.html") {
      return;
    }
    const hash = window.location.hash.replace(/^#/, "").trim();
    if (!hash) {
      return;
    }
    const selector = `#${util.escapeSelector(hash)}`;
    const target =
      document.querySelector(selector) ||
      document.querySelector(`[data-anchor="${util.escapeSelector(hash)}"]`);
    if (target) {
      target.scrollIntoView({ block: "start", behavior: "smooth" });
    }
  }

  function init() {
    prepareDirectoryPageAnchors();
    scrollToDirectoryPageHash();
    window.addEventListener("hashchange", scrollToDirectoryPageHash);
  }

  return { init };
}
