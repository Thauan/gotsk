"use client"

import { useEffect, useState } from "react"
import { Filter, MoreHorizontal, PauseCircle, PlayCircle, RefreshCw, Search, XCircle } from "lucide-react"
import { Badge } from "./ui/badge"
import { Button } from "./ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./ui/dropdown-menu"
import { Input } from "./ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "./ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./ui/tabs"
import { TaskDetailPanel } from "./task-detail-panel"

const mockStats = {
  processed: 1245,
  failed: 28,
  pending: 67,
  retrying: 12,
  scheduled: 43,
}

const mockQueues = [
  { name: "default", count: 45 },
  { name: "mailers", count: 12 },
  { name: "critical", count: 23 },
  { name: "low", count: 30 },
]

const statusColors = {
  pending: "bg-yellow-500",
  processing: "bg-blue-500",
  completed: "bg-green-500",
  failed: "bg-red-500",
  retrying: "bg-purple-500",
  scheduled: "bg-slate-500",
}

type Task = {
  id: string
  name: string
  status: "pending" | "processing" | "completed" | "failed" | "retrying" | "scheduled"
  payload: Record<string, string | number | boolean | null | object>
}


export function TaskDashboard() {
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [activeTab, setActiveTab] = useState("all")
  const [searchQuery, setSearchQuery] = useState("")
  const [tasks, setTasks] = useState<Task[]>([])


  useEffect(() => {
    const sse = new EventSource("http://localhost:8080/sse")
    sse.onmessage = (event) => {
      const incoming: Task = JSON.parse(event.data)
  
      console.log({ incoming })
      setTasks((prev) => {
          const index = prev.findIndex((t) => t.id === incoming.id)
    
          if (index !== -1) {
            const updated = [...prev]
            updated[index] = incoming
            return updated
          }
    
          return [incoming, ...prev]
      });
    }
    return () => sse.close()
  }, [])
  
  // useEffect(() => {
  //   const ws = new WebSocket("ws://localhost:8080/ws")
  
  //   ws.onmessage = (event) => {
  //     const incoming: Task = JSON.parse(event.data)
  
  //     console.log({ incoming })
  //     setTasks((prev) => {
  //       const index = prev.findIndex((t) => t.id === incoming.id)
  
  //       if (index !== -1) {
  //         const updated = [...prev]
  //         updated[index] = incoming
  //         return updated
  //       }
  
  //       return [incoming, ...prev]
  //     })
  //   }
  
  //   ws.onerror = (err) => {
  //     console.error("WebSocket error:", err)
  //   }
  
  //   return () => ws.close()
  // }, [])
  

  const filteredTasks = tasks.filter((task) => {
    if (activeTab !== "all" && task.status !== activeTab) return false
  
    if (
      searchQuery &&
      !task.name.toLowerCase().includes(searchQuery.toLowerCase()) &&
      !task.id.toLowerCase().includes(searchQuery.toLowerCase())
    ) {
      return false
    }
  
    return true
  })

  return (
    <div className="flex min-h-screen bg-muted/40">
      <div className="hidden w-64 flex-col border-r bg-background p-4 md:flex">
        <div className="flex items-center gap-2 py-4">
          <RefreshCw className="h-5 w-5 text-primary" />
          <h1 className="text-xl font-bold">Gotsk</h1>
        </div>

        <div className="mt-6">
          <h2 className="mb-2 text-sm font-semibold">Filas</h2>
          <div className="space-y-1">
            {mockQueues.map((queue) => (
              <div key={queue.name} className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-muted">
                <span className="text-sm">{queue.name}</span>
                <Badge variant="secondary">{queue.count}</Badge>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-6">
          <h2 className="mb-2 text-sm font-semibold">Filtros</h2>
          <div className="space-y-1">
            <div className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-muted">
              <span className="text-sm">Últimas 24h</span>
            </div>
            <div className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-muted">
              <span className="text-sm">Últimos 7 dias</span>
            </div>
            <div className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-muted">
              <span className="text-sm">Personalizado</span>
            </div>
          </div>
        </div>

        <div className="mt-auto">
          <Button variant="outline" className="w-full">
            <RefreshCw className="mr-2 h-4 w-4" />
            Atualizar dados
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-auto p-4 md:p-6">
        <div className="mx-auto max-w-7xl">

          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Processadas</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{mockStats.processed}</div>
                <div className="text-xs text-muted-foreground">+12% em relação à semana passada</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Falhas</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{mockStats.failed}</div>
                <div className="text-xs text-muted-foreground">-3% em relação à semana passada</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Pendentes</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{mockStats.pending}</div>
                <div className="text-xs text-muted-foreground">+5% em relação à semana passada</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Retentativas</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{mockStats.retrying}</div>
                <div className="text-xs text-muted-foreground">+2% em relação à semana passada</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Agendadas</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{mockStats.scheduled}</div>
                <div className="text-xs text-muted-foreground">+8% em relação à semana passada</div>
              </CardContent>
            </Card>
          </div>

          <div className="mt-6">
            <Tabs defaultValue="all" value={activeTab} onValueChange={setActiveTab}>
              <div className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
                <TabsList>
                  <TabsTrigger value="all">Todas</TabsTrigger>
                  <TabsTrigger value="pending">Pendentes</TabsTrigger>
                  <TabsTrigger value="processing">Processando</TabsTrigger>
                  <TabsTrigger value="completed">Concluídas</TabsTrigger>
                  <TabsTrigger value="failed">Falhas</TabsTrigger>
                </TabsList>
                <div className="flex w-full items-center gap-2 sm:w-auto">
                  <div className="relative flex-1 sm:w-64">
                    <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="Buscar tarefas..."
                      className="pl-8"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                    />
                  </div>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" size="icon">
                        <Filter className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent className="bg-white" align="end">
                      <DropdownMenuItem>Filtrar por fila</DropdownMenuItem>
                      <DropdownMenuItem>Filtrar por classe</DropdownMenuItem>
                      <DropdownMenuItem>Filtrar por data</DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>

              <TabsContent value={activeTab} className="mt-4">
                <Card>
                  <CardContent className="p-0">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-[100px]">Status</TableHead>
                          <TableHead>ID / Nome</TableHead>
                          <TableHead className="text-right">Ações</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {filteredTasks?.map((task) => (
                          <TableRow key={task.id} onClick={() => setSelectedTask(task)} className="cursor-pointer">
                            <TableCell>
                              <div className="flex items-center gap-2">
                                <span
                                  className={`h-2 w-2 rounded-full ${statusColors[task.status as keyof typeof statusColors]}`}
                                />
                                <span className="capitalize">{task.status}</span>
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="font-medium">{task.name}</div>
                              <div className="text-xs text-muted-foreground">{task.id}</div>
                            </TableCell>
                            <TableCell className="text-right">
                              <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                  <Button variant="ghost" size="icon">
                                    <MoreHorizontal className="h-4 w-4" />
                                    <span className="sr-only">Abrir menu</span>
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent className="bg-white" align="end">
                                  <DropdownMenuItem>Ver detalhes</DropdownMenuItem>
                                  {task.status === "pending" && (
                                    <DropdownMenuItem>
                                      <PlayCircle className="mr-2 h-4 w-4" />
                                      Executar agora
                                    </DropdownMenuItem>
                                  )}
                                  {task.status === "processing" && (
                                    <DropdownMenuItem>
                                      <PauseCircle className="mr-2 h-4 w-4" />
                                      Pausar
                                    </DropdownMenuItem>
                                  )}
                                  {task.status === "failed" && (
                                    <DropdownMenuItem>
                                      <RefreshCw className="mr-2 h-4 w-4" />
                                      Tentar novamente
                                    </DropdownMenuItem>
                                  )}
                                  <DropdownMenuItem className="text-destructive">
                                    <XCircle className="mr-2 h-4 w-4" />
                                    Cancelar
                                  </DropdownMenuItem>
                                </DropdownMenuContent>
                              </DropdownMenu>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          </div>
        </div>
      </div>

      {selectedTask && <TaskDetailPanel task={selectedTask} onClose={() => setSelectedTask(null)} />}
    </div>
  )
}
