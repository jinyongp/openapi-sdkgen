<script setup lang="ts">
import { computed, type ComputedRef } from "vue";
import { useRoute } from "vitepress";
import DefaultTheme from "vitepress/theme";
import DocSelectors from "./components/DocSelectors.vue";
import { parseDocContext, type DocContext } from "../doc-context";
const route: ReturnType<typeof useRoute> = useRoute();
const scope: ComputedRef<string> = computed((): string => {
  const context: DocContext | null = parseDocContext(route.path);
  return `${context?.locale ?? "en"}/${context?.sdk ?? "root"}`;
});
</script>
<template>
  <DefaultTheme.Layout :key="scope">
    <template #nav-bar-content-after><DocSelectors placement="desktop" /></template>
    <template #nav-screen-content-after><DocSelectors placement="mobile" /></template>
  </DefaultTheme.Layout>
</template>
