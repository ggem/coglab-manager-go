import type { ReactElement, ReactNode } from 'react'
import { render, type RenderOptions } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'

interface Options extends Omit<RenderOptions, 'wrapper'> {
  // Only components that call a react-router hook (useNavigate,
  // useParams, ...) need this -- omit it and no MemoryRouter is mounted.
  route?: string
}

// Mirrors internal/httpapi/helpers_test.go's newTestServer/
// newAuthenticatedTestServer on the Go side: one place that wires up the
// providers a component actually needs, rather than repeating
// QueryClientProvider/MemoryRouter boilerplate in every test file.
export function renderWithProviders(ui: ReactElement, { route, ...options }: Options = {}) {
  const queryClient = new QueryClient({
    defaultOptions: {
      // Retries would make a mocked-rejection test wait through backoff
      // delays before the error actually surfaces.
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  function Wrapper({ children }: { children: ReactNode }) {
    const content = <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    return route === undefined ? content : <MemoryRouter initialEntries={[route]}>{content}</MemoryRouter>
  }

  return render(ui, { wrapper: Wrapper, ...options })
}
