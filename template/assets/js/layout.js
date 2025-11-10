export function createLayoutModule({ dom }) {
  const summaryNav = dom.qs(".summary-panel");
  const contentContainer = dom.qs(".content");
  let summaryPlaceholder = null;
  let sectionObserver = null;

  function relocateSummary() {
    if (!summaryNav || !contentContainer) {
      return;
    }
    const isNarrow = window.matchMedia("(max-width: 1000px)").matches;
    if (isNarrow) {
      if (!summaryPlaceholder) {
        summaryPlaceholder = document.createComment("summary-anchor");
        summaryNav.parentNode?.insertBefore(summaryPlaceholder, summaryNav);
      }
      const breadcrumb = contentContainer.querySelector(".path");
      if (breadcrumb) {
        breadcrumb.insertAdjacentElement("afterend", summaryNav);
      } else if (contentContainer.firstChild !== summaryNav) {
        contentContainer.insertBefore(summaryNav, contentContainer.firstChild);
      }
      summaryNav.classList.add("summary-inline");
    } else if (summaryPlaceholder && summaryPlaceholder.parentNode) {
      summaryPlaceholder.parentNode.insertBefore(summaryNav, summaryPlaceholder.nextSibling);
      summaryNav.classList.remove("summary-inline");
    } else {
      summaryNav.classList.remove("summary-inline");
    }
  }

  function ensureSentinel() {
    if (!contentContainer) {
      return null;
    }
    let element = contentContainer.querySelector("[data-summary-sentinel]");
    if (!element) {
      element = document.createElement("div");
      element.setAttribute("data-summary-sentinel", "true");
      element.setAttribute("aria-hidden", "true");
      element.style.position = "relative";
      element.style.width = "1px";
      element.style.height = "1px";
      element.style.marginTop = "-1px";
      element.style.pointerEvents = "none";
      contentContainer.insertBefore(element, contentContainer.firstChild);
    }
    return { element };
  }

  function observeSections() {
    if (!contentContainer) {
      return;
    }
    const summary = document.querySelector(".summary");
    if (!summary) {
      return;
    }
    const links = dom.qsa("a[data-section]", summary);
    if (!links.length) {
      return;
    }
    const map = new Map();
    links.forEach((link) => {
      const selector = link.getAttribute("data-section");
      if (!selector) {
        return;
      }
      const target = document.querySelector(selector);
      if (target) {
        map.set(target, link);
      }
    });
    if (!map.size) {
      return;
    }
    const sentinel = ensureSentinel();
    if (sentinel) {
      map.set(sentinel.element, links[0]);
    }
    const state = new Map();
    const chooseActive = () => {
      const visible = Array.from(state.entries())
        .filter(([, info]) => info.isIntersecting)
        .sort((a, b) => a[1].top - b[1].top);
      const candidate = visible.find(([, info]) => info.top >= -16) || visible[0];
      if (!candidate) {
        return;
      }
      const link = map.get(candidate[0]);
      if (link) {
        links.forEach((item) => item.classList.toggle("active", item === link));
      }
    };
    sectionObserver = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!map.has(entry.target)) {
            return;
          }
          state.set(entry.target, {
            isIntersecting: entry.isIntersecting,
            top: entry.boundingClientRect.top,
          });
        });
        chooseActive();
      },
      { rootMargin: "0px 0px -70% 0px", threshold: 0.2 }
    );
    map.forEach((_, element) => sectionObserver.observe(element));
    links.forEach((link) => link.classList.remove("active"));
    const first = links[0];
    if (first) {
      first.classList.add("active");
    }
  }

  function refreshSections() {
    if (sectionObserver) {
      sectionObserver.disconnect();
      sectionObserver = null;
    }
    observeSections();
  }

  function init() {
    relocateSummary();
    observeSections();
    window.addEventListener("resize", relocateSummary);
  }

  return { init, refreshSections };
}
