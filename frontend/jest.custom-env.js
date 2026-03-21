/**
 * Custom Jest environment that extends jsdom and adds all Web APIs that MSW v2 requires.
 */
const JSDOMEnvironment = require('jest-environment-jsdom').default;
const { TextDecoder, TextEncoder } = require('util');
const { ReadableStream, WritableStream, TransformStream } = require('stream/web');

class CustomJSDOMEnvironment extends JSDOMEnvironment {
  constructor(config, context) {
    super(config, context);

    // Inject globals into this.global in constructor so they are available
    // when modules are first loaded (before setup() is called)
    this._injectWebGlobals();
  }

  _injectWebGlobals() {
    const g = this.global;

    // Text encoding
    if (!g.TextDecoder) g.TextDecoder = TextDecoder;
    if (!g.TextEncoder) g.TextEncoder = TextEncoder;

    // Streams
    if (!g.ReadableStream) g.ReadableStream = ReadableStream;
    if (!g.WritableStream) g.WritableStream = WritableStream;
    if (!g.TransformStream) g.TransformStream = TransformStream;

    // Node 18+ globals — available on the outer Node.js global at require time
    // (before jsdom context takes over)
    const nodeGlobals = ['fetch', 'Request', 'Response', 'Headers',
      'FormData', 'Blob', 'File', 'URLSearchParams', 'URL',
      'AbortController', 'AbortSignal', 'Event', 'EventTarget',
      'BroadcastChannel', 'MessageChannel', 'MessageEvent',
      'structuredClone', 'crypto',
    ];

    for (const name of nodeGlobals) {
      if (!g[name] && typeof global[name] !== 'undefined') {
        g[name] = global[name];
      }
    }
  }

  async setup() {
    await super.setup();
    // Re-inject in case super.setup() cleared anything
    this._injectWebGlobals();
  }
}

module.exports = CustomJSDOMEnvironment;
