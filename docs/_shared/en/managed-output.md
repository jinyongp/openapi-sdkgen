The manifest in the output directory records the files owned by the generator.
An incremental run:

- keeps unchanged generated files in place;
- atomically replaces changed files;
- removes only stale files previously owned by the manifest;
- preserves files you added outside the manifest;
- stops if an owned generated file was edited, the manifest is invalid, a new
  generated path conflicts with an unmanaged file, or another writer holds the
  output lock.
