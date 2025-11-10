export function createHelpers() {
  return {
    setHint(element, message = "", isError = false) {
      if (!element) {
        return;
      }
      element.textContent = message || "";
      element.classList.toggle("error", Boolean(isError && message));
    },
    debounce(callback, delay) {
      let timer = null;
      return (...args) => {
        if (timer) {
          window.clearTimeout(timer);
        }
        timer = window.setTimeout(() => {
          callback(...args);
        }, delay);
      };
    },
    formatDate(value) {
      if (!value) {
        return "";
      }
      let date = null;
      if (value instanceof Date) {
        date = value;
      } else if (typeof value === "number") {
        date = new Date(value * 1000);
      } else {
        date = new Date(String(value));
      }
      if (!date || Number.isNaN(date.getTime())) {
        return String(value);
      }
      const monthNames = [
        "Jan",
        "Feb",
        "Mar",
        "Apr",
        "May",
        "Jun",
        "Jul",
        "Aug",
        "Sep",
        "Oct",
        "Nov",
        "Dec",
      ];
      const month = monthNames[date.getMonth()];
      const day = date.getDate();
      const hours = String(date.getHours()).padStart(2, "0");
      const minutes = String(date.getMinutes()).padStart(2, "0");
      const seconds = String(date.getSeconds()).padStart(2, "0");
      let tz = "";
      try {
        if (
          typeof Intl !== "undefined" &&
          typeof Intl.DateTimeFormat === "function" &&
          typeof Intl.DateTimeFormat.prototype.formatToParts === "function"
        ) {
          const parts = new Intl.DateTimeFormat("en-US", {
            timeZoneName: "short",
          }).formatToParts(date);
          const tzPart = parts.find((item) => item.type === "timeZoneName");
          if (tzPart?.value) {
            tz = tzPart.value;
          }
        }
      } catch (_error) {
        tz = "";
      }
      if (!tz) {
        const offsetMinutes = -date.getTimezoneOffset();
        const sign = offsetMinutes >= 0 ? "+" : "-";
        const abs = Math.abs(offsetMinutes);
        const hh = String(Math.floor(abs / 60)).padStart(2, "0");
        const mm = String(abs % 60).padStart(2, "0");
        tz = `GMT${sign}${hh}${mm}`;
      }
      const year = date.getFullYear();
      return `${month} ${day} ${hours}:${minutes}:${seconds} ${tz} ${year}`;
    },
    slugify(value) {
      if (!value) {
        return "";
      }
      return String(value)
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "");
    },
    escapeSelector(value) {
      if (typeof value !== "string") {
        return "";
      }
      if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
        return CSS.escape(value);
      }
      return value.replace(/([ !"#$%&'()*+,./:;<=>?@[\\\]^`{|}~])/g, "\\$1");
    },
  };
}
