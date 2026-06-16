export type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type QueueBuckets = Record<string, number>

export type QueueLongContextTier = {
  threshold_tokens: number
  max_running: number
  lease_turns: number
  lease_idle_timeout_seconds: number
}

export type QueueLongContextTierStatus = QueueLongContextTier & {
  running: number
  queued: number
}

export type QueueTimeSlotConfig = {
  start_time: string
  end_time: string
  weekdays?: number[]
  enabled: boolean
  max_queue_size: number
  queue_timeout: number
  long_context_tiers: QueueLongContextTier[]
}

export type QueueModelSnapshot = {
  queued: number
  avg_wait_sec: number
  max_wait_sec: number
  throughput_rpm: number
  max_queue_size: number
  enabled: boolean
  buckets: QueueBuckets
  long_context_tiers: QueueLongContextTierStatus[]
}

export type QueueStatusSnapshot = {
  queue_enabled: boolean
  total_queued: number
  models: Record<string, QueueModelSnapshot>
}

export type QueueModelStatus = QueueModelSnapshot & {
  model_name: string
}

export type QueueLongContextTaskKind = 'queued' | 'leased'

export type QueueLongContextTask = {
  id: string
  kind: QueueLongContextTaskKind
  model_name: string
  token_id: number
  company_id: number
  department_id?: number
  company_name?: string
  department_name?: string
  threshold_tokens: number
  estimated_prompt_tokens: number
  priority: number
  status: string
  created_at: number
  wait_seconds: number
  remaining_turns: number
  lease_turns: number
  idle_timeout_seconds: number
  idle_expires_at?: number
  active: boolean
}

export type QueueLongContextTasksSnapshot = {
  items: QueueLongContextTask[]
  total: number
}

export type QueueConfig = {
  model_name: string
  enabled: boolean
  max_queue_size: number
  queue_timeout: number
  long_context_tiers: QueueLongContextTier[]
  time_slots: QueueTimeSlotConfig[]
}

export type QueueConfigFormData = {
  model_name: string
  enabled: boolean
  max_queue_size: number
  queue_timeout: number
  long_context_tiers: QueueLongContextTier[]
  time_slots: QueueTimeSlotConfig[]
}
