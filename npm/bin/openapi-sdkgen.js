#!/usr/bin/env node

import { readFile } from "node:fs/promises";

import { runLauncher } from "../lib/launcher.js";

const packageJSON = JSON.parse(
  await readFile(new URL("../package.json", import.meta.url), "utf8"),
);

try {
  process.exitCode = await runLauncher({
    version: packageJSON.version,
    args: process.argv.slice(2),
  });
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`openapi-sdkgen: ${message}`);
  process.exitCode = 1;
}
