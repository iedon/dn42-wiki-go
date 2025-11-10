export function createExternalLinksModule() {
  function init() {
    const anchors = Array.from(document.querySelectorAll("a[href]"));
    if (!anchors.length) {
      return;
    }
    const currentOrigin = window.location.origin;
    anchors.forEach((anchor) => {
      const href = anchor.getAttribute("href");
      if (!href || anchor.dataset.externalSkip === "true") {
        return;
      }
      try {
        const url = new URL(href, window.location.href);
        if (url.origin === currentOrigin) {
          return;
        }
      } catch (_error) {
        return;
      }
      anchor.setAttribute("target", "_blank");
      anchor.classList.add("external-link");
    });
  }

  return { init };
}
