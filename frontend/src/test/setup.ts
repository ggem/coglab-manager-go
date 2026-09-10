import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// Unmounts any rendered component between tests -- without this, a
// component that reads window.location or sets up a subscription in one
// test can leak into the next one's DOM.
afterEach(() => {
  cleanup()
})
