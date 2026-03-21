/**
 * Custom Jest resolver to ensure MSW always uses pre-compiled CJS files,
 * not TypeScript source files.
 */
module.exports = (request, options) => {
  // If the request is for an MSW TypeScript source file, redirect to the compiled JS
  // This handles cases where ts-jest resolves .ts over .js for msw internals
  const mswSrcPattern = /node_modules[/\\]msw[/\\]src[/\\](.+?)\.ts$/;
  const match = request.match(mswSrcPattern);
  if (match) {
    const relativePath = match[1];
    // Map src/core/... -> lib/core/...
    const compiled = request.replace(
      /node_modules[/\\]msw[/\\]src[/\\]/,
      'node_modules/msw/lib/'
    ).replace(/\.ts$/, '.js');
    try {
      return options.defaultResolver(compiled, options);
    } catch (_) {
      // Fall through to default resolution
    }
  }

  // Block until-async and other ESM-only packages that don't have CJS builds
  // by trying to use their main field first
  return options.defaultResolver(request, {
    ...options,
    packageFilter: (pkg) => {
      // Prefer 'main' (CJS) over 'module' (ESM) for jest
      if (pkg.main && !pkg.main.endsWith('.mjs')) {
        return {
          ...pkg,
          module: pkg.main,
        };
      }
      return pkg;
    },
  });
};
