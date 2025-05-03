"use client"
//@ts-nocheck
import { X } from "lucide-react"
import { Button } from "./ui/button"
import { ScrollArea } from "./ui/scroll-area"
import { Separator } from "./ui/separator"

type Task = {
  id: string
  name: string
  payload: Record<string, string | number | boolean | null | object>
  status: string
}

interface TaskDetailPanelProps {
  task: Task
  onClose: () => void
}

export function TaskDetailPanel({ task, onClose }: TaskDetailPanelProps) {
  return (
    <div className="fixed inset-y-0 right-0 z-50 w-full max-w-md border-l bg-background bg-white shadow-lg sm:w-96">
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="text-lg font-semibold">Detalhes da Tarefa</h2>
          <Button variant="ghost" size="icon" onClick={onClose}>
            <X className="h-4 w-4" />
            <span className="sr-only">Fechar</span>
          </Button>
        </div>

        <ScrollArea className="flex-1 p-4">
          <div className="space-y-6">
            <div>
              <h3 className="text-sm font-medium text-muted-foreground">ID</h3>
              <p className="mt-1">{task.id}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-muted-foreground">Nome</h3>
              <p className="mt-1">{task.name}</p>
            </div>

            {/* <div>
              <h3 className="text-sm font-medium text-muted-foreground">Classe</h3>
              <p className="mt-1">{task.class}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-muted-foreground">Fila</h3>
              <p className="mt-1">{task.queue}</p>
            </div> */}

            <div>
              <h3 className="text-sm font-medium text-muted-foreground">Status</h3>
              <p className="mt-1 capitalize">{task.status}</p>
            </div>

            {/* <div>
              <h3 className="text-sm font-medium text-muted-foreground">Tentativas</h3>
              <p className="mt-1">{task.attempts}</p>
            </div> */}

            {/* <div>
              <h3 className="text-sm font-medium text-muted-foreground">Criado em</h3>
              <p className="mt-1">{new Date(task.createdAt).toLocaleString()}</p>
            </div> */}

            {/* <div>
              <h3 className="text-sm font-medium text-muted-foreground">Atualizado em</h3>
              <p className="mt-1">{new Date(task.updatedAt).toLocaleString()}</p>
            </div> */}

            <Separator />

            <div>
              <h3 className="text-sm font-medium text-muted-foreground">Argumentos</h3>
              <pre className="mt-2 rounded-md bg-muted p-4 text-xs">
                {typeof task.payload === "string" 
                  ? JSON.stringify(JSON.parse(task.payload), null, 2) 
                  : JSON.stringify(task.payload, null, 2)}
              </pre>
            </div>

            {task.status === "failed" && (
              <div>
                <h3 className="text-sm font-medium text-muted-foreground">Erro</h3>
                <pre className="mt-2 rounded-md bg-red-50 p-4 text-xs text-red-500 dark:bg-red-950/30">
                  Error: Failed to process task due to invalid input format. at processTask (worker.js:42:15) at runJob
                  (queue.js:128:22)
                </pre>
              </div>
            )}
          </div>
        </ScrollArea>

        <div className="border-t p-4">
          <div className="flex gap-2">
            {task.status === "failed" && <Button className="flex-1">Tentar novamente</Button>}
            {task.status === "pending" && <Button className="flex-1">Executar agora</Button>}
            {task.status === "processing" && <Button className="flex-1">Pausar</Button>}
            <Button variant="outline" className="flex-1">
              Cancelar
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
