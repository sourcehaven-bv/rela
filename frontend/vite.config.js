import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath, URL } from 'node:url';
// Get API base URL from environment variable or default to localhost:8080
// For e2e tests, VITE_API_BASE is set by global-setup.ts
const apiBase = process.env.VITE_API_BASE || 'http://localhost:8080';
export default defineConfig(({ mode }) => {
    // Log at config evaluation time so we can see what port is being used
    console.error(`[vite.config] API proxy target: ${apiBase}`);
    // __E2E_TEST_HOOKS__ gates test-only knobs (e.g. the backtick-autocomplete
    // delay override) so they are tree-shaken out of production bundles but
    // compiled in for the E2E suite. It is true only for the dev-mode build
    // (`vite build --mode development`, i.e. `npm run build:e2e`); the default
    // production `vite build` runs in 'production' mode, leaving it false.
    // import.meta.env.DEV can't be used: it is false for ANY `vite build`
    // regardless of --mode, so it would strip the hooks even from the E2E
    // bundle. See issue #890.
    const e2eTestHooks = mode === 'development';
    return {
        plugins: [vue()],
        base: '/',
        define: {
            __E2E_TEST_HOOKS__: JSON.stringify(e2eTestHooks),
        },
        build: {
            outDir: '../internal/dataentry/static/v2',
            emptyOutDir: true,
        },
        resolve: {
            alias: {
                '@': fileURLToPath(new URL('./src', import.meta.url)),
            },
        },
        server: {
            port: 5173,
            proxy: {
                '/api': {
                    target: apiBase,
                    changeOrigin: true,
                },
            },
        },
    };
});
