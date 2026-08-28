import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The Go API has no /api prefix (see internal/httpapi/server.go), so the
// dev proxy forwards each of its top-level route prefixes individually
// rather than proxying everything -- add an entry here when a new
// top-level resource is added there. Proxying (rather than enabling CORS
// on the Go side) also means the browser sees same-origin requests, so the
// session cookie works without any SameSite/CORS credential wrangling.
const apiPaths = [
  '/healthz',
  '/login',
  '/logout',
  '/families',
  '/guardians',
  '/children',
  '/labs',
  '/conditions',
  '/condition-values',
  '/equipment',
  '/experiment-roles',
  '/experiments',
]

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: Object.fromEntries(
      apiPaths.map((path) => [path, { target: 'http://localhost:8080', changeOrigin: true }]),
    ),
  },
})
