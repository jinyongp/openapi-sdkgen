import type { Theme } from "vitepress";
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
    app.component("CompatibilityResults", CompatibilityResults);
    app.component("GraphSelection", GraphSelection);
  },
} satisfies Theme;
