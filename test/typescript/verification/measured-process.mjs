import fs from "node:fs";
import { spawn } from "node:child_process";

/** Owns the timing wrapper and its workload until completion or termination. */
export async function runMeasured(
  command,
  args,
  { cwd, resourceFile, timeout, maxBuffer = 16 * 1024 * 1024 },
) {
  const timed = fs.existsSync("/usr/bin/time");
  const grouped = process.platform !== "win32";
  const child = spawn(
    timed ? "/usr/bin/time" : command,
    timed
      ? [
          "-f",
          '{"userSeconds":%U,"systemSeconds":%S,"peakRSSKiB":%M}',
          "-o",
          resourceFile,
          command,
          ...args,
        ]
      : args,
    { cwd, detached: grouped, stdio: ["ignore", "pipe", "pipe"] },
  );
  const terminate = () => {
    if (child.pid === undefined) return;
    try {
      if (grouped) process.kill(-child.pid, "SIGKILL");
      else child.kill("SIGKILL");
    } catch (error) {
      if (error.code !== "ESRCH") throw error;
    }
  };
  const interrupts = new Map([
    ["SIGINT", 130],
    ["SIGTERM", 143],
  ]);
  const handlers = new Map(
    [...interrupts].map(([signal, code]) => [
      signal,
      () => {
        terminate();
        process.exit(code);
      },
    ]),
  );
  for (const [signal, handler] of handlers) process.once(signal, handler);
  process.once("exit", terminate);
  try {
    return await new Promise((resolve) => {
      const stdout = [],
        stderr = [];
      let bytes = 0,
        failure;
      const timer = setTimeout(() => {
        failure = Object.assign(new Error("Measured workload exceeded its deadline"), {
          code: "ETIMEDOUT",
        });
        terminate();
      }, timeout);
      const collect = (target) => (chunk) => {
        bytes += chunk.length;
        if (bytes > maxBuffer) {
          failure = Object.assign(new Error("Measured workload exceeded its output limit"), {
            code: "ENOBUFS",
          });
          terminate();
        } else target.push(chunk);
      };
      child.stdout.on("data", collect(stdout));
      child.stderr.on("data", collect(stderr));
      child.on("error", (error) => {
        failure = error;
        terminate();
      });
      child.on("close", (status, signal) => {
        clearTimeout(timer);
        resolve({
          status,
          signal,
          error: failure,
          stdout: Buffer.concat(stdout).toString(),
          stderr: Buffer.concat(stderr).toString(),
          resources:
            status === 0 && timed ? JSON.parse(fs.readFileSync(resourceFile, "utf8")) : null,
        });
      });
    });
  } finally {
    for (const [signal, handler] of handlers) process.removeListener(signal, handler);
    process.removeListener("exit", terminate);
  }
}
