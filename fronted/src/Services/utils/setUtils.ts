/**
 * Devuelve un Set nuevo con value agregado o quitado. Si se indica maxSize,
 * no agrega cuando el Set ya está lleno.
 */
export function toggleInSet<T>(set: Set<T>, value: T, maxSize = Infinity): Set<T> {
  const next = new Set(set);

  if (next.has(value)) {
    next.delete(value);
  } else if (next.size < maxSize) {
    next.add(value);
  }

  return next;
}
