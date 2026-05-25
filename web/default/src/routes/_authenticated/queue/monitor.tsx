import { createFileRoute } from '@tanstack/react-router'
import { QueueMonitorPage } from '@/features/queue/components/queue-monitor-page'

export const Route = createFileRoute('/_authenticated/queue/monitor')({
  component: QueueMonitorPage,
})
