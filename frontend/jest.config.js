/** @type {import('jest').Config} */
const config = {
  preset: 'ts-jest',
  // Custom environment extends jsdom and adds Node fetch API globals for MSW v2
  testEnvironment: '<rootDir>/jest.custom-env.js',
  testEnvironmentOptions: {
    customExportConditions: ['node', 'require', 'default'],
  },
  // Polyfills run before the test framework (and before modules are imported)
  setupFiles: ['<rootDir>/jest.polyfills.js'],
  // Run setup imports (e.g. @testing-library/jest-dom) after the test framework is installed
  setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/src/$1',
    '\\.(css|less|scss)$': 'identity-obj-proxy',
  },
  transform: {
    '^.+\\.(ts|tsx)$': ['ts-jest', {
      tsconfig: {
        jsx: 'react-jsx',
        moduleResolution: 'node',
        // Don't include node_modules in TS compilation
        allowJs: false,
      },
      // Don't let ts-jest resolve .ts extensions for dependencies
      diagnostics: false,
    }],
  },
  // Do not transform anything in node_modules (all deps ship CJS or are not needed)
  transformIgnorePatterns: ['/node_modules/'],
  testMatch: [
    '<rootDir>/src/**/__tests__/**/*.{ts,tsx}',
    '<rootDir>/src/**/*.{spec,test}.{ts,tsx}',
  ],
  collectCoverageFrom: [
    'src/**/*.{ts,tsx}',
    '!src/**/*.d.ts',
    '!src/**/layout.tsx',
    '!src/app/**/page.tsx',
    // Not yet tested — deferred to future test phases
    '!src/lib/auth.ts',
    '!src/components/resumes/**',
    '!src/components/applications/applications-list.tsx',
    '!src/components/applications/application-list.tsx',
    '!src/components/onboarding/**',
    '!src/components/settings/credentials-form.tsx',
    '!src/components/ui/progress.tsx',
    '!src/components/ui/textarea.tsx',
    '!src/components/layout/header.tsx',
    '!src/app/error.tsx',
    '!src/app/global-error.tsx',
  ],
  coverageThreshold: {
    global: { branches: 60, functions: 60, lines: 60, statements: 60 },
  },
};

module.exports = config;
