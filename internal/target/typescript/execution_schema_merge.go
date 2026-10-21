package typescript

// Ordinals come from the Go planner's sorted descriptor names. Merging these
// streams preserves insertion order without JS string sorting or a superset map.
const executionSchemaMergeSource = `import type { WireSchema, WireSchemas } from '../runtime/schema/wire-types.js'

export type SchemaEntry = readonly [ordinal: number, name: string, schema: WireSchema];
export type SchemaBlock = readonly SchemaEntry[];
interface Cursor { readonly block: SchemaBlock; index: number }
type SchemaPair = readonly [name: string, schema: WireSchema];

function ordinal(cursor: Cursor): number { return cursor.block[cursor.index]![0]; }

export function mergeSchemas(blocks: readonly SchemaBlock[]): WireSchemas {
  const heap: Cursor[] = [];
  const pairs: SchemaPair[] = [];
  for (const block of blocks) {
    if (block.length === 0) continue;
    const cursor: Cursor = { block, index: 0 };
    let child: number = heap.length;
    heap.push(cursor);
    while (child > 0) {
      const parent: number = (child - 1) >>> 1;
      const ancestor: Cursor = heap[parent]!;
      if (ordinal(ancestor) <= block[0]![0]) break;
      heap[child] = ancestor;
      child = parent;
    }
    heap[child] = cursor;
  }
  let previous: number = -1;
  while (heap.length > 0) {
    const cursor: Cursor = heap[0]!;
    const entry: SchemaEntry = cursor.block[cursor.index]!;
    if (entry[0] <= previous) throw new Error('Duplicate or unsorted execution schema entry');
    previous = entry[0];
    pairs.push([entry[1], entry[2]]);
    cursor.index++;
    if (cursor.index === cursor.block.length) {
      const last: Cursor = heap.pop()!;
      if (heap.length === 0) break;
      heap[0] = last;
    }
    const root: Cursor = heap[0]!;
    let parent: number = 0;
    while (parent * 2 + 1 < heap.length) {
      let child: number = parent * 2 + 1;
      const right: number = child + 1;
      if (right < heap.length && ordinal(heap[right]!) < ordinal(heap[child]!)) child = right;
      if (ordinal(root) <= ordinal(heap[child]!)) break;
      heap[parent] = heap[child]!;
      parent = child;
    }
    heap[parent] = root;
  }
  return Object.fromEntries(pairs);
}
`
