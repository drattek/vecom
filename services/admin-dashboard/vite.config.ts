import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, 'src'),
      },
    },
    server: {
      proxy: {
        // core-orchestrator sirve HTTPS en el puerto 443 (SERVER_PORT en .env,
        // mismo mapeo en docker-compose). El certificado es para vecom.odo.mx,
        // así que `secure: false` evita el fallo de verificación contra localhost.
        // Sobrescribe el destino con VITE_API_PROXY_TARGET si lo corres en otro puerto/host.
        '/api': {
          target: env.VITE_API_PROXY_TARGET || 'https://localhost:443',
          changeOrigin: true,
          secure: false,
        },
      },
    },
  }
})
