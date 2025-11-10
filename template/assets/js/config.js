export function createConfig(root) {
  const dataset = root?.dataset ?? {};
  const basePath = normalizeBase(dataset.base ?? "");
  return {
    basePath,
    searchEnabled: dataset.searchEnabled === "true",
    editable: dataset.editable === "true",
    pagePath: dataset.path ?? "",
    repoUrl: dataset.repo ?? "",
    searchIndexPath: (dataset.searchIndex ?? "").trim(),
    legacySearchPath: (dataset.search ?? "").trim(),
  };
}

function normalizeBase(raw) {
  if (!raw) {
    return "";
  }
  let value = String(raw).trim();
  if (!value || value === "/") {
    return "";
  }
  if (/^https?:\/\//i.test(value)) {
    try {
      const parsed = new URL(value, window.location.origin);
      value = parsed.pathname ?? "";
    } catch (_error) {
      value = "";
    }
  }
  value = value.replace(/\/{2,}/g, "/").replace(/\/+$/, "");
  if (!value) {
    return "";
  }
  if (!value.startsWith("/")) {
    value = `/${value}`;
  }
  return value === "/" ? "" : value;
}
