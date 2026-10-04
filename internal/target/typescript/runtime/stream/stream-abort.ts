/** Waits for a stream operation while observing caller cancellation. */
export function awaitAbortable<Value>(
  value: Promise<Value>,
  signal: AbortSignal | undefined,
): Promise<Value> {
  if (signal === undefined) return value;
  if (signal.aborted) {
    void value.catch((): undefined => undefined);
    return Promise.reject(signal.reason);
  }
  return new Promise(
    (
      resolve: (value: Value | PromiseLike<Value>) => void,
      reject: (reason?: unknown) => void,
    ): void => {
      const onAbort: () => void = (): void => reject(signal.reason);
      signal.addEventListener("abort", onAbort, { once: true });
      value.then(
        (result: Value): void => {
          signal.removeEventListener("abort", onAbort);
          resolve(result);
        },
        (cause: unknown): void => {
          signal.removeEventListener("abort", onAbort);
          reject(cause);
        },
      );
    },
  );
}
