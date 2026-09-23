import { readFile } from "node:fs/promises";

const [path] = process.argv.slice(2);
if (!path) {
  console.error("usage: vuln-sarif-report.mjs <govulncheck.sarif>");
  process.exit(2);
}

let document;
try {
  document = JSON.parse(await readFile(path, "utf8"));
} catch (error) {
  console.error(
    `govulncheck returned invalid SARIF: ${error instanceof Error ? error.message : String(error)}`,
  );
  process.exit(2);
}

if (!Array.isArray(document?.runs)) {
  console.error("govulncheck returned invalid SARIF: missing runs");
  process.exit(2);
}

const results = document.runs.flatMap((run) =>
  Array.isArray(run?.results) ? run.results : [],
);
if (results.length === 0) {
  console.log("ok govulncheck: no reachable known vulnerabilities");
  process.exit(0);
}

console.error(
  `govulncheck found ${results.length} reachable known vulnerabilit${results.length === 1 ? "y" : "ies"}:`,
);
for (const result of results.slice(0, 40)) {
  const id =
    typeof result?.ruleId === "string" && result.ruleId !== ""
      ? result.ruleId
      : "unknown";
  const message =
    typeof result?.message?.text === "string" && result.message.text !== ""
      ? result.message.text.replace(/\s+/g, " ").trim()
      : "known vulnerability";
  console.error(`- ${id}: ${message}`);
}
if (results.length > 40) {
  console.error(`- ... ${results.length - 40} additional result(s) omitted`);
}
process.exit(1);
