export function createSidebarModule({ dom, modal }) {
  const toggle = dom.qs("[data-sidebar-toggle]");
  const modalElement = dom.qs("#sidebar-modal");
  const container = modalElement?.querySelector("[data-sidebar-modal-container]");
  const sidebar = dom.qs(".sidebar.sidebar-extra");

  if (!toggle || !modalElement || !container || !sidebar) {
    return { init() {} };
  }

  let placeholder = null;
  let isInModal = false;

  function ensurePlaceholder() {
    if (!placeholder) {
      placeholder = document.createComment("sidebar-placeholder");
      sidebar.parentNode?.insertBefore(placeholder, sidebar);
    }
    return placeholder;
  }

  function moveSidebarToModal() {
    if (isInModal) {
      return;
    }
    const anchor = ensurePlaceholder();
    if (!anchor || !container) {
      return;
    }
    container.appendChild(sidebar);
    sidebar.classList.add("sidebar-modal__content");
    sidebar.setAttribute("data-sidebar-overlay", "true");
    toggle.setAttribute("aria-expanded", "true");
    isInModal = true;
  }

  function restoreSidebar() {
    if (!isInModal) {
      toggle.setAttribute("aria-expanded", "false");
      return;
    }
    const anchor = ensurePlaceholder();
    const parent = anchor?.parentNode;
    if (parent) {
      parent.insertBefore(sidebar, anchor.nextSibling);
    }
    sidebar.classList.remove("sidebar-modal__content");
    sidebar.removeAttribute("data-sidebar-overlay");
    toggle.setAttribute("aria-expanded", "false");
    isInModal = false;
  }

  function handleToggle(event) {
    event.preventDefault();
    moveSidebarToModal();
    modal.open(modalElement, { trigger: toggle, stack: false });
  }

  function handleModalOpen(event) {
    if (event.target !== modalElement) {
      return;
    }
    moveSidebarToModal();
  }

  function handleModalClose(event) {
    if (event.target !== modalElement) {
      return;
    }
    restoreSidebar();
  }

  function handleMediaChange(event) {
    const isNarrow = event.matches;
    toggle.hidden = !isNarrow;
    if (!isNarrow) {
      if (modalElement.classList.contains("active")) {
        modal.close(modalElement);
      } else {
        restoreSidebar();
      }
    }
  }

  function init() {
    toggle.setAttribute("aria-expanded", "false");
    toggle.addEventListener("click", handleToggle);
    modalElement.addEventListener("modal:open", handleModalOpen);
    modalElement.addEventListener("modal:close", handleModalClose);
  }

  return { init };
}
