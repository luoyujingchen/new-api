export type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type QueueBuckets = Record<string, number>

export type QueueLongContextTier = {
  threshold_tokens: number
  max_running: number
}

export type QueueLongContextTierStatus = QueueLongContextTier & {
  running: number
  queued: number
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

export type QueueConfig = {
  model_name: string
  enabled: boolean
  max_queue_size: number
  queue_timeout: number
  long_context_tiers: QueueLongContextTier[]
}

export type QueueConfigFormData = {
  model_name: string
  enabled: boolean
  max_queue_size: number
  queue_timeout: number
  long_context_tiers: QueueLongContextTier[]
}
