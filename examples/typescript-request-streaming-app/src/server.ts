import {
  createServer,
  type Server,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import { createInterface, type Interface } from "node:readline";

import type { TodoImportItem } from "./todo.js";

const port: number = Number(process.env.IMPORT_API_PORT ?? "18792");
const server: Server = createServer(
  async (request: IncomingMessage, response: ServerResponse): Promise<void> => {
    if (request.method !== "POST" || request.url !== "/todos/import") {
      response.writeHead(404).end();
      return;
    }
    if (request.headers["content-type"] !== "application/x-ndjson") {
      response.writeHead(415).end();
      return;
    }
    const lines: Interface = createInterface({
      input: request,
      crlfDelay: Infinity,
    });
    let imported: number = 0;
    try {
      for await (const line of lines) {
        if (!line.trim()) continue;
        const item: TodoImportItem = JSON.parse(line) as TodoImportItem;
        if (typeof item?.title !== "string") throw new Error("Invalid title");
        console.log(`Received: ${item.title}`);
        imported++;
      }
      response
        .writeHead(200, { "content-type": "application/json" })
        .end(JSON.stringify({ imported }));
    } catch {
      response.writeHead(400).end();
    } finally {
      lines.close();
    }
  },
);
server.listen(port, "127.0.0.1", (): void => {
  console.log(`Import API listening at http://127.0.0.1:${port}`);
});
