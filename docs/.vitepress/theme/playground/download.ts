import { strToU8, zip, type Zippable } from "fflate";
import type { GeneratedArtifact } from "./wasm";

export function archiveArtifacts(artifacts: readonly GeneratedArtifact[]): Promise<Uint8Array<ArrayBuffer>> {
  const files: Zippable = Object.create(null);
  for (const artifact of artifacts) {
    files[artifact.path] = strToU8(artifact.content);
  }
  return new Promise((resolve: (value: Uint8Array<ArrayBuffer>) => void, reject: (reason: Error) => void): void => {
    zip(files, { level: 6 }, (error: Error | null, data: Uint8Array<ArrayBuffer>): void => {
      if (error) reject(error);
      else resolve(data);
    });
  });
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url: string = URL.createObjectURL(blob);
  const link: HTMLAnchorElement = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.append(link);
  link.click();
  link.remove();
  window.setTimeout((): void => URL.revokeObjectURL(url), 60_000);
}
