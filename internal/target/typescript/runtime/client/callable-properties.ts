/**
 * Adds namespace members without colliding with Function prototype properties.
 * Generated runtime callables may expose only capabilities in their public type
 * (for example, stream-only operations). Decorating that surface requires an
 * object, not a public buffered-call signature, and must not add such a signature.
 */
export function assignCallableProperties<Call extends object, Members extends object>(
  call: Call,
  members: Members,
): Call & Members {
  for (const [key, value] of Object.entries(members)) {
    Object.defineProperty(call, key, {
      value,
      enumerable: true,
      configurable: true,
      writable: true,
    });
  }
  return call as Call & Members;
}
