<script setup lang="ts">
import { onMounted } from "vue";
import { legacyDestination } from "../../legacy-destination";
import { withDocsBase } from "../../doc-context";
interface LegacyProps { readonly route: string; }
const props: LegacyProps = defineProps<LegacyProps>();
onMounted((): void => {
  const destination: URL = new URL(withDocsBase(legacyDestination(props.route, window.location.href)), window.location.origin);
  if (!destination.pathname.endsWith("/") && !destination.pathname.endsWith(".html")) destination.pathname += ".html";
  window.location.replace(destination.href);
});
</script>
<template><span hidden></span></template>
