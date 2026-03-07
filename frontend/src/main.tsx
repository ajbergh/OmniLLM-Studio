import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { initAPIBase } from './api'
import { ThemeProvider } from './theme'

// In the Wails desktop build, discover the local API server URL before
// rendering so that all fetch() / SSE calls use the real HTTP server.
initAPIBase().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ThemeProvider>
        <App />
      </ThemeProvider>
    </StrictMode>,
  )
})
