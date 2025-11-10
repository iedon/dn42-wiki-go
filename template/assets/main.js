import { createConfig } from "./js/config.js";
import { createDomUtils } from "./js/dom.js";
import { createHelpers } from "./js/helpers.js";
import { createApi } from "./js/api.js";
import { createModalManager } from "./js/modal.js";
import { createLayoutModule } from "./js/layout.js";
import { createDirectoryModule } from "./js/directory.js";
import { createExternalLinksModule } from "./js/external-links.js";
import { createBackToTopModule } from "./js/back-to-top.js";
import { createSearchModule } from "./js/search.js";
import { createHistoryModule } from "./js/history.js";
import { createEditorModule } from "./js/editor.js";
import { createToolbarModule } from "./js/toolbar.js";

const body = document.body;
if (!body) {
  throw new Error("Expected document body to be present");
}

const config = createConfig(body);
const dom = createDomUtils();
const helpers = createHelpers();
const api = createApi(config);

const modal = createModalManager(dom);
const layout = createLayoutModule({ dom });
const directory = createDirectoryModule({ config, dom, helpers });
const externalLinks = createExternalLinksModule();
const backToTop = createBackToTopModule(dom, () => layout.refreshSections());
const search = createSearchModule({ config, dom, api, helpers });
const history = createHistoryModule({ config, dom, api, helpers, modal });
const editor = createEditorModule({ config, dom, api, helpers, modal });
const toolbar = createToolbarModule({ dom, config, editor, history });

modal.init();
externalLinks.init();
layout.init();
directory.init();
backToTop.init();
search.init();
history.init();
editor.init();
toolbar.init();
