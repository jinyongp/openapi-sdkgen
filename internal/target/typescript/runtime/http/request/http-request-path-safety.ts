/** Rejects dot segments that could change the normalized operation route. */
export function assertSafeOperationPath(path: string): void {
  for (const segment of path.split("/")) {
    const dots: string = segment.toLowerCase().replaceAll("%2e", ".");
    if (dots === "." || dots === "..")
      throw new TypeError(
        "Operation path contains a URL dot-segment after parameter serialization",
      );
  }
}
