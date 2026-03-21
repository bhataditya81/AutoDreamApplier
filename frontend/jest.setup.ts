import '@testing-library/jest-dom';

// Polyfill Web APIs needed by MSW v2 in jsdom environment
import { TextDecoder, TextEncoder } from 'util';
import { ReadableStream } from 'stream/web';

Object.assign(globalThis, {
  TextDecoder,
  TextEncoder,
  ReadableStream,
  // Node 18+ has native fetch; ensure they are in the global scope for jsdom
  fetch: global.fetch,
  Request: global.Request,
  Response: global.Response,
  Headers: global.Headers,
});
