import { createReadStream, type ReadStream } from "node:fs";
import { createInterface, type Interface } from "node:readline";
import { createClient } from "./generated/api/index.js";

import type { TodoImportItem } from "./todo.js";

async function* readTodos(file: string): AsyncGenerator<TodoImportItem> {
  const input: ReadStream = createReadStream(file);
  const lines: Interface = createInterface({ input, crlfDelay: Infinity });
  try {
    for await (const line of lines) {
      if (line.trim()) yield JSON.parse(line) as TodoImportItem;
    }
  } finally {
    lines.close();
    input.destroy();
  }
}

const baseURL: string =
  process.env.IMPORT_API_BASE_URL ?? "http://127.0.0.1:18792";
const api: ReturnType<typeof createClient> = createClient({ baseURL });
const result: Awaited<ReturnType<typeof api.$operations.importTodos>> =
  await api.$operations.importTodos({ body: readTodos("./todos.ndjson") });
console.log(`Imported ${result.imported} todos`);
