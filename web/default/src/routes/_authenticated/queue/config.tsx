import { createFileRoute } from '@tanstack/react-router'
import { QueueConfigPage } from '@/features/queue/components/queue-config-page'

export const Route = createFileRoute('/_authenticated/queue/config')({
  component: QueueConfigPage,
})