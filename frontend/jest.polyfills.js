/**
 * Polyfills for jsdom test environment.
 * Plain .js so it runs immediately without compilation.
 * These run BEFORE test framework and before module loading.
 *
 * In Jest's jsdom environment, Node's built-in globals (fetch, Response, etc.)
 * are not available because jsdom replaces the global scope. We restore them here.
 */
const { TextDecoder, TextEncoder } = require('util');
const { ReadableStream } = require('stream/web');

// Use Object.defineProperty to avoid errors if already defined
function safeDefine(key, value) {
  if (value === undefined) return;
  try {
    Object.defineProperty(globalThis, key, {
      value,
      writable: true,
      configurable: true,
    });
  } catch (_) {
    globalThis[key] = value;
  }
}

safeDefine('TextDecoder', TextDecoder);
safeDefine('TextEncoder', TextEncoder);
safeDefine('ReadableStream', ReadableStream);

// In Node 18+, fetch API is available. We need to get it from the Node.js internals.
// process.binding is not available, but we can get Response from a dynamic eval in
// the parent (non-jsdom) context.
// The cleanest way: use the jest globalSetup or a custom environment.
// Workaround: require from jest-environment-node before jsdom overwrites globals.

// The polyfill file runs in the test worker context where jsdom has set up its globals.
// We need to grab the Node-native fetch classes BEFORE jsdom runs.
// Since we can't do that here (jsdom already initialized), use a workaround:
// Check if they exist in the process itself via process.binding or vm.
try {
  const vm = require('vm');
  const nodeContext = vm.createContext({});
  // In Node 18+, built-in fetch is available in a fresh VM context
  const script = new vm.Script('({Response, Request, Headers, fetch})');
  const result = script.runInContext(nodeContext);
  // result will be undefined fields if not available
  if (result.Response) safeDefine('Response', result.Response);
  if (result.Request) safeDefine('Request', result.Request);
  if (result.Headers) safeDefine('Headers', result.Headers);
  if (result.fetch) safeDefine('fetch', result.fetch);
} catch (_) {
  // VM approach failed, try alternatives
}

// Last resort: try to extract from the module cache or process
if (typeof globalThis.Response === 'undefined') {
  try {
    // jest-environment-node has access to the Node globals; we can steal from there
    // by creating a Node environment and extracting its globals
    const { Response, Request, Headers, fetch } = require('@jest/globals') || {};
    if (Response) safeDefine('Response', Response);
    if (Request) safeDefine('Request', Request);
    if (Headers) safeDefine('Headers', Headers);
    if (fetch) safeDefine('fetch', fetch);
  } catch (_) {
    // ignore
  }
}
