/**
 * The subscribe-and-notify half of a store.
 *
 * Held rather than inherited: the stores that need it disagree on what they
 * publish — a value for some, a bare signal for others — and one of them keeps
 * a separate set per scope. Composition covers all three; a shared base class
 * would have to make the unkeyed stores carry a key they do not have.
 */
export class Listeners<T = void> {
  readonly #listeners = new Set<(value: T) => void>();

  /** Registers a subscriber and returns the call that removes it. */
  add(listener: (value: T) => void): () => void {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  }

  /** Lets a store holding one set per scope drop a scope nobody watches. */
  get size(): number {
    return this.#listeners.size;
  }

  emit(value: T): void {
    for (const listener of this.#listeners) listener(value);
  }
}
