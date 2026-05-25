import { api } from '@/lib/api'
import type {
  ApiResponse,
  QueueConfig,
  QueueConfigFormData,
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
