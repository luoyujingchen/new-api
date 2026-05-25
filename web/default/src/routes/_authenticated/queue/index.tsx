import { createFileRoute, Navigate } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/queue/')({
  component: () => <Navigate to="/queue/monitor" />,
})
