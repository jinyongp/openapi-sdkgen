import type { Theme } from "vitepress";
import SDKHome from "./components/SDKHome.vue";
import SDKOverview from "./components/SDKOverview.vue";
import LegacyRedirect from "./components/LegacyRedirect.vue";
import Layout from "./Layout.vue";
import DefaultTheme from "vitepress/theme";
import Playground from "./components/Playground.vue";
import CompatibilityResults from "./components/CompatibilityResults.vue";
import GraphSelection from "./components/GraphSelection.vue";
import "./style.css";

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }): void {
    app.component("Playground", Playground);
    app.component("SDKHome", SDKHome);
    app.component("SDKOverview", SDKOverview);
    app.component("LegacyRedirect", LegacyRedirect);
    app.component("CompatibilityResults", CompatibilityResults);
    app.component("GraphSelection", GraphSelection);
  },
} satisfies Theme;
