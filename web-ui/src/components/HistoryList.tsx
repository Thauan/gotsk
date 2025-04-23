/* eslint-disable @typescript-eslint/no-explicit-any */
type Task = {
  id: string
  name: string
  status: string
  payload: Record<string, any>
  createdAt: string
}

export function HistoryList({ tasks }: { tasks: Task[] }) {
  if (tasks.length === 0) return <p>Nenhuma tarefa encontrada.</p>

  return (
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Nome</th>
          <th>Status</th>
          <th>Payload</th>
          <th>Data</th>
        </tr>
      </thead>
      <tbody>
        {tasks.map((task) => (
          <tr key={task.id}>
            <td>{task.id}</td>
            <td>{task.name}</td>
            <td>{task.status}</td>
            <td>
              <pre>{JSON.stringify(task.payload, null, 2)}</pre>
            </td>
            <td>{new Date(task.createdAt).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
  