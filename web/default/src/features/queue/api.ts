import { api } from '@/lib/api'
import type {
  ApiResponse,
  QueueConfig,
  QueueConfigFormData,
  QueueLongContextTaskKind,
  QueueLongContextTasksSnapshot,
  QueueModelStatus,
  QueueStatusSnapshot,
} from './types'

export async function getQueueStatus(): Promise<QueueStatusSnapshot> {
  const res = await api.get<ApiResponse<QueueStatusSnapshot>>('/api/queue/status')
  return res.data.data
}

export async function getQueueModelStatus(
  modelName: string
): Promise<QueueModelStatus> {
  const res = await api.get<ApiResponse<QueueModelStatus>>(
    `/api/queue/status/${encodeURIComponent(modelName)}`
  )
  return res.data.data
}

export async function getQueueLongContextTasks(
  modelName?: string
): Promise<QueueLongContextTasksSnapshot> {
  const params = new URLSearchParams()
  if (modelName) {
    params.set('model_name', modelName)
  }
  const query = params.toString()
  const res = await api.get<ApiResponse<QueueLongContextTasksSnapshot>>(
    `/api/queue/long-context${query ? `?${query}` : ''}`
  )
  return res.data.data
}

export async function cancelQueueLongContextTask(
  kind: QueueLongContextTaskKind,
  id: string
): Promise<ApiResponse<{ cancelled: boolean }>> {
  const res = await api.post<ApiResponse<{ cancelled: boolean }>>(
    '/api/queue/long-context/cancel',
    { kind, id }
  )
  return res.data
}

export async function getQueueConfigs(): Promise<QueueConfig[]> {
  const res = await api.get<ApiResponse<QueueConfig[]>>('/api/queue/config')
  return res.data.data
}

export async function upsertQueueConfig(
  modelName: string,
  data: Omit<QueueConfigFormData, 'model_name'>
): Promise<ApiResponse<QueueConfig>> {
  const res = await api.put<ApiResponse<QueueConfig>>(
    `/api/queue/config/${encodeURIComponent(modelName)}`,
    data
  )
  return res.data
}

export async function deleteQueueConfig(
  modelName: string
): Promise<ApiResponse<null>> {
  const res = await api.delete<ApiResponse<null>>(
    `/api/queue/config/${encodeURIComponent(modelName)}`
  )
  return res.data
}
