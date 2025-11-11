export function createModalManager(domUtils) {
  const backdrop = domUtils.qs("#modal-backdrop");
  const stack = [];
  const triggers = new WeakMap();

  function dispatchLifecycle(modal, phase) {
    if (!modal) {
      return;
    }
    try {
      modal.dispatchEvent(
        new CustomEvent(`modal:${phase}`, {
          bubbles: true,
          detail: { modal },
        })
      );
    } catch (_error) {
      /* no-op */
    }
  }

  function hideModal(modal) {
    domUtils.hide(modal);
  }

  function showModal(modal) {
    domUtils.show(modal);
    dispatchLifecycle(modal, "open");
    focusFirstControl(modal);
  }

  function focusFirstControl(modal) {
    if (!modal) {
      return;
    }
    const focusable = modal.querySelector(
      '[data-autofocus], input, textarea, select, button, a[href], [tabindex]:not([tabindex="-1"])'
    );
    if (focusable && typeof focusable.focus === "function") {
      focusable.focus();
    } else if (typeof modal.focus === "function") {
      modal.setAttribute("tabindex", "-1");
      modal.focus();
    }
  }

  function trackTrigger(modal, trigger) {
    const active = trigger instanceof HTMLElement ? trigger : document.activeElement;
    if (modal && active) {
      triggers.set(modal, active);
    }
  }

  function restoreFocus(modal) {
    if (!modal) {
      return;
    }
    const trigger = triggers.get(modal);
    if (trigger && typeof trigger.focus === "function") {
      trigger.focus();
    }
    triggers.delete(modal);
  }

  function activateBackdrop() {
    domUtils.show(backdrop);
    document.body.classList.add("modal-open");
  }

  function deactivateBackdrop() {
    if (!stack.length) {
      domUtils.hide(backdrop);
      document.body.classList.remove("modal-open");
    }
  }

  function close(modal) {
    if (!modal) {
      return;
    }
    const index = stack.lastIndexOf(modal);
    if (index !== -1) {
      stack.splice(index, 1);
    }
    hideModal(modal);
    dispatchLifecycle(modal, "close");
    restoreFocus(modal);
    deactivateBackdrop();
  }

  function closeTop() {
    if (!stack.length) {
      return;
    }
    const modal = stack[stack.length - 1];
    close(modal);
  }

  function closeAll() {
    while (stack.length) {
      close(stack[stack.length - 1]);
    }
  }

  function open(modal, { trigger = null, stack: stacked = false } = {}) {
    if (!modal) {
      return;
    }
    if (!stacked) {
      closeAll();
    }
    if (!stack.includes(modal)) {
      stack.push(modal);
    }
    trackTrigger(modal, trigger);
    showModal(modal);
    activateBackdrop();
  }

  function init() {
    domUtils.hide(backdrop);
    domUtils.qsa(".modal").forEach((modal) => hideModal(modal));
    if (backdrop) {
      backdrop.addEventListener("click", () => closeTop());
    }
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeTop();
      }
    });
    domUtils.qsa("[data-close]").forEach((button) => {
      button.addEventListener("click", (event) => {
        event.preventDefault();
        close(button.closest(".modal"));
      });
    });
  }

  return { init, open, close, closeTop, closeAll };
}
