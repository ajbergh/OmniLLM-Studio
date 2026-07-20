import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { ToolApprovalCenter } from './components/ToolApprovalCenter'
import { initAPIBase } from './api'
import { installClientContextFetch } from './clientContextFetch'
import { ThemeProvider } from './theme'
import { MotionConfig } from 'framer-motion'

// Add browser locale/timezone context before any API request is made.
installClientContextFetch()

// In the Wails desktop build, discover the local API server URL before
// rendering so that all fetch() / SSE calls use the real HTTP server.
initAPIBase().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ThemeProvider>
        <MotionConfig reducedMotion="user">
          <App />
          <ToolApprovalCenter />
        </MotionConfig>
      </ThemeProvider>
    </StrictMode>,
  )
})
