export function createBackToTopModule(dom, refreshSections) {
  const button = dom.qs(".back-to-top");

  function toggleButton() {
    if (!button) {
      return;
    }
    if (window.scrollY > 200) {
      button.classList.add("back-to-top--active");
    } else {
      button.classList.remove("back-to-top--active");
    }
  }

  function init() {
    if (!button) {
      return;
    }
    window.addEventListener("scroll", toggleButton, { passive: true });
    toggleButton();
    button.addEventListener("click", () => {
      window.scrollTo({ top: 0, behavior: "smooth" });
      refreshSections();
    });
  }

  return { init };
}
