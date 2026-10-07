import DefaultTheme from "vitepress/theme";
import Playground from "./components/Playground.vue";
import CompatibilityResults from "./components/CompatibilityResults.vue";
import GraphSelection from "./components/GraphSelection.vue";
import "./style.css";

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component("Playground", Playground);
    app.component("CompatibilityResults", CompatibilityResults);
    app.component("GraphSelection", GraphSelection);
  },
};
