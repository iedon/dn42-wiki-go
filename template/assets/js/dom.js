export function createDomUtils() {
  return {
    qs(selector, scope = document) {
      return scope.querySelector(selector);
    },
    qsa(selector, scope = document) {
      return Array.from(scope.querySelectorAll(selector));
    },
    show(element) {
      if (!element) {
        return;
      }
      element.classList.remove("hidden");
      element.classList.add("active");
      if (element.classList.contains("modal")) {
        element.style.display = "flex";
      } else if (element.id === "modal-backdrop") {
        element.style.display = "block";
      } else {
        element.style.display = "";
      }
      element.setAttribute("aria-hidden", "false");
    },
    hide(element) {
      if (!element) {
        return;
      }
      element.classList.remove("active");
      element.classList.add("hidden");
      if (element.classList.contains("modal")) {
        element.style.display = "none";
      } else if (element.id === "modal-backdrop") {
        element.style.display = "none";
      } else {
        element.style.display = "";
      }
      element.setAttribute("aria-hidden", "true");
    },
    toggleClass(element, className, force) {
      element?.classList.toggle(className, force);
    },
  };
}
