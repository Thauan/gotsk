import { useEffect, useState } from 'react'
import { HistoryList } from './components/HistoryList'

function App() {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [history, setHistory] = useState<any[]>([])

  useEffect(() => {
    // Função que busca o histórico
    const fetchHistory = () => {
      fetch('/api/history')
        .then((res) => res.json())
        .then((data) => {
          setHistory(data)
        })
        .catch((err) => {
          console.error('Erro ao buscar histórico:', err)
          setHistory([])
        })
    }

    fetchHistory()

    const interval = setInterval(fetchHistory, 2000)

    return () => clearInterval(interval)
  }, [])

  return (
    <div style={{ padding: 24 }}>
      <h1>📜 Histórico de Tarefas</h1>
      <HistoryList tasks={history} />
    </div>
  )
}

export default App
