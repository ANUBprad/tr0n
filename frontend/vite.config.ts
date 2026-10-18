import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  define: { 'process.env.NODE_ENV': '"production"' },
  plugins: [tailwindcss()],
  build: {
    outDir: '../internal/ui/hero-assets',
    emptyOutDir: true,
    lib: { entry: 'src/main.tsx', formats: ['es'], fileName: () => 'hero.js', cssFileName: 'hero' },
  },
});
