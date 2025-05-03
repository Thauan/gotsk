import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { TaskDashboard } from '../src/components/task-dashboard'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <TaskDashboard />
  </StrictMode>,
)
