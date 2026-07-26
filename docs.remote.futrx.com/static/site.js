import { initializeCodeCopy } from "./modules/code-copy.js";
import { initializeDiagrams } from "./modules/diagrams.js";
import { initializeNavigation } from "./modules/navigation.js";
import { initializeSearch } from "./modules/search.js";
import { initializeTableOfContents } from "./modules/table-of-contents.js";
import { initializeTheme } from "./modules/theme.js";

initializeTheme();
initializeNavigation();
initializeSearch();
initializeCodeCopy();
initializeTableOfContents();
await initializeDiagrams();
