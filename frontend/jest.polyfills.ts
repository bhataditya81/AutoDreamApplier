/**
 * Polyfills for jsdom test environment.
 * These run BEFORE the test framework and modules are loaded.
 */
import { TextDecoder, TextEncoder } from 'util';
import { ReadableStream } from 'stream/web';

Object.defineProperty(globalThis, 'TextDecoder', { value: TextDecoder, writable: true });
Object.defineProperty(globalThis, 'TextEncoder', { value: TextEncoder, writable: true });
Object.defineProperty(globalThis, 'ReadableStream', { value: ReadableStream, writable: true });

// Node 18+ has native fetch — expose to the jsdom global scope
if (typeof globalThis.Response === 'undefined' && typeof global.Response !== 'undefined') {
  Object.defineProperty(globalThis, 'Response', { value: global.Response, writable: true });
}
if (typeof globalThis.Request === 'undefined' && typeof global.Request !== 'undefined') {
  Object.defineProperty(globalThis, 'Request', { value: global.Request, writable: true });
}
if (typeof globalThis.Headers === 'undefined' && typeof global.Headers !== 'undefined') {
  Object.defineProperty(globalThis, 'Headers', { value: global.Headers, writable: true });
}
if (typeof globalThis.fetch === 'undefined' && typeof global.fetch !== 'undefined') {
  Object.defineProperty(globalThis, 'fetch', { value: global.fetch, writable: true });
}
