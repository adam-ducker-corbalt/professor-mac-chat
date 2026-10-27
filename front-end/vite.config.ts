import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ command }) => ({
  plugins: [react()],
  // Like ProfessorMAC, the Go server serves built assets under /static/.
  base: command === 'build' ? '/static/' : '/',
  // Builds to front-end/dist; `make build` copies it into cmd/pmac-chat/static,
  // as ProfessorMAC's server.Dockerfile does for cmd/pmac/static.
  build: {
    target: 'es2022',
  },
  server: {
    // For when the Go server has API routes: the dev page calls /api on :8080.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
}));
