export function createToolbarModule({ dom, config: runtime, editor, history }) {
  function init() {
    dom.qsa("[data-action]").forEach((button) => {
      button.addEventListener("click", (event) => {
        event.preventDefault();
        const action = button.getAttribute("data-action");
        switch (action) {
          case "edit":
            editor.openEdit(button);
            break;
          case "new":
            editor.openNew(button);
            break;
          case "rename":
            editor.openRename(button);
            break;
          case "history":
            history.open(button);
            break;
          case "home":
            window.location.href = runtime.basePath || "/";
            break;
          default:
            break;
        }
      });
    });
  }

  return { init };
}
