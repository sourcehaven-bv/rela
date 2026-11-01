/**
 * How long a search-as-you-type box waits after the last keystroke before it
 * asks the server (TKT-T2I7P2).
 *
 * At 150 ms a typist with about 200 ms between keys sent a request for most
 * keys and aborted nearly all of them. Each abort still cost the server the
 * started query and a pooled database connection. At 300 ms that typist sends
 * about one request per pause, and the final result arrives about 120 ms
 * later than at 150 ms. A slower typist (about 280 ms between keys) still has
 * pauses longer than 300 ms, so some aborts remain.
 *
 * This module imports nothing: the app editor bundles the mention menu,
 * which reads it, into a dependency-free script.
 */
export const SEARCH_DEBOUNCE_MS = 300
