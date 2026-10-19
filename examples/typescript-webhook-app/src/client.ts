import { createReceiver } from "./worker.js";

const router: ReturnType<typeof createReceiver> = createReceiver(
  "local-example-token",
);

async function send(token: string, body: unknown): Promise<number> {
  const response: Response = await router.fetch(
    new Request("http://localhost/hooks/todos/completed", {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-webhook-token": token,
      },
      body: JSON.stringify(body),
    }),
  );
  return response.status;
}

interface DeliveryResults {
  readonly accepted: number;
  readonly invalidCredentials: number;
  readonly invalidBody: number;
}

const results: DeliveryResults = {
  accepted: await send("local-example-token", {
    todoID: "todo-1",
    completed: true,
  }),
  invalidCredentials: await send("wrong-token", {
    todoID: "todo-1",
    completed: true,
  }),
  invalidBody: await send("local-example-token", { todoID: "todo-1" }),
};

console.log(JSON.stringify(results, null, 2));
