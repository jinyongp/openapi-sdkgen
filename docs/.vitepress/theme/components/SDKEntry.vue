<script setup lang="ts">
import { computed, onMounted, type ComputedRef } from "vue";
import { useData } from "vitepress";
import { contextDestination, withDocsBase, type DisplayLocale, type SDKTarget } from "../../doc-context";

interface SDKEntryProps { readonly sdk: SDKTarget; }
const props: SDKEntryProps = defineProps<SDKEntryProps>();
const { lang } = useData();
const korean: ComputedRef<boolean> = computed((): boolean => lang.value === "ko-KR");
const destination: ComputedRef<string> = computed((): string => {
  const locale: DisplayLocale = korean.value ? "ko" : "en";
  return `${withDocsBase(contextDestination({ sdk: props.sdk, locale, topic: "getting-started" }, props.sdk, locale).path)}.html`;
});
onMounted((): void => window.location.replace(destination.value));
</script>

<template>
  <a :href="destination">{{ korean ? '시작하기' : 'Getting started' }}</a>
</template>
