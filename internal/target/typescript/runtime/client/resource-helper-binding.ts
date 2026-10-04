/** Preserves the exact Link helper object while binding a resource path. */
export function bindResourceLinks(bound: object, operation: object): unknown {
  const links: unknown = Reflect.get(operation, "links");
  return links === undefined ? bound : Object.assign(bound, { links });
}

/** Preserves the exact pagination helper while binding a resource path. */
export function bindResourcePagination(bound: object, operation: object): unknown {
  const paginate: unknown = Reflect.get(operation, "paginate");
  return paginate === undefined ? bound : Object.assign(bound, { paginate });
}
