export interface GeneratedArtifact {
  path: string;
  content: string;
}

export interface GenerationResult {
  artifacts?: GeneratedArtifact[];
  diagnostics?: string;
  error?: string;
}

export interface GenerationOptions {
  goImportPath?: string;
  goPackage?: string;
  addons?: string[];
}

interface GenerationRequest {
  id: number;
  source: string;
  target: string;
  options: GenerationOptions;
}

interface GenerationResponse {
  id: number;
  result?: GenerationResult;
  error?: string;
}

interface PendingGeneration {
  resolve: (result: GenerationResult) => void;
  reject: (error: Error) => void;
}

let generatorWorker: Worker | undefined;
let nextRequestID: number = 0;
const pendingGenerations: Map<number, PendingGeneration> = new Map<number, PendingGeneration>();

function rejectPending(error: Error): void {
  pendingGenerations.forEach((pending: PendingGeneration): void => pending.reject(error));
  pendingGenerations.clear();
  generatorWorker?.terminate();
  generatorWorker = undefined;
}

function worker(): Worker {
  if (generatorWorker) return generatorWorker;

  const instance: Worker = new Worker(`${import.meta.env.BASE_URL}playground/generator-worker.js`);
  generatorWorker = instance;
  instance.addEventListener("message", (event: MessageEvent<GenerationResponse>): void => {
    if (generatorWorker !== instance) return;
    const pending: PendingGeneration | undefined = pendingGenerations.get(event.data.id);
    if (!pending) return;
    pendingGenerations.delete(event.data.id);
    if (event.data.error) {
      pending.reject(new Error(event.data.error));
      return;
    }
    pending.resolve(event.data.result ?? {});
  });
  instance.addEventListener("error", (): void => {
    if (generatorWorker !== instance) return;
    rejectPending(new Error("Generator worker stopped unexpectedly."));
  });
  return instance;
}

export function generate(source: string, target: string, options: GenerationOptions = {}): Promise<GenerationResult> {
  return new Promise((resolve: (result: GenerationResult) => void, reject: (error: Error) => void): void => {
    const id: number = nextRequestID;
    nextRequestID += 1;
    pendingGenerations.set(id, { resolve, reject });
    const request: GenerationRequest = { id, source, target, options };
    try { worker().postMessage(request); } catch (cause) { rejectPending(cause instanceof Error ? cause : new Error(String(cause))); }
  });
}

export function cancelGeneration(): void { rejectPending(new Error("Generation canceled.")); }
