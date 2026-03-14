import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath, URL } from 'node:url';
export default defineConfig(function (_a) {
    var command = _a.command;
    return ({
        plugins: [vue()],
        // Use /v2/ base in production build, root in dev for simpler URLs
        base: command === 'build' ? '/v2/' : '/',
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
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
            },
        },
    });
});
