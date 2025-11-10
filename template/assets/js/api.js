const API_CONTENT_TYPE = "application/json";

export function createApi(runtime) {
  const { basePath } = runtime;

  function apiPath(path) {
    const clean = path.startsWith("/") ? path : `/${path}`;
    return basePath ? `${basePath}${clean}` : clean;
  }

  function toRoute(input) {
    if (!input) {
      return "/";
    }
    let route = String(input).trim().replace(/\\/g, "/");
    if (!route.startsWith("/")) {
      route = `/${route}`;
    }
    return route.replace(/\/{2,}/g, "/");
  }

  function pageUrl(pathValue) {
    const route = toRoute(pathValue);
    return basePath ? `${basePath}${route}` : route;
  }

  function absoluteUrl(pathValue) {
    if (!pathValue) {
      return window.location.href;
    }
    if (/^https?:/i.test(pathValue)) {
      return pathValue;
    }
    const base = window.location.origin.replace(/\/+$/, "");
    if (pathValue.startsWith("/")) {
      return `${base}${pathValue}`;
    }
    return `${base}/${pathValue}`;
  }

  async function fetchJSON(path, options = {}) {
    const { method = "GET", body, headers = {} } = options;
    const url = /^https?:/i.test(path) ? path : apiPath(path);
    const requestHeaders = new Headers(headers);
    if (!requestHeaders.has("Content-Type") && body) {
      requestHeaders.set("Content-Type", API_CONTENT_TYPE);
    }
    const response = await fetch(url, {
      method,
      body,
      headers: requestHeaders,
    });
    if (!response.ok) {
      let message = `${response.status} ${response.statusText}`;
      try {
        const data = await response.clone().json();
        if (data && typeof data.error === "string") {
          message = data.error;
        }
      } catch (_error) {
        // ignore JSON parse failure
      }
      throw new Error(message);
    }
    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes(API_CONTENT_TYPE)) {
      return response.json();
    }
    const text = await response.text();
    if (!text) {
      return {};
    }
    try {
      return JSON.parse(text);
    } catch (_error) {
      throw new Error("Invalid JSON response");
    }
  }

  return { apiPath, fetchJSON, pageUrl, toRoute, absoluteUrl };
}
