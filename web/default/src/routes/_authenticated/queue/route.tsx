import { createFileRoute, redirect } from '@tanstack/react-router'
import { ROLE } from '@/lib/roles'
import { QueueLayout } from '@/features/queue'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/queue')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (auth.user?.role !== ROLE.SUPER_ADMIN && auth.user?.role !== ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: QueueLayout,
})
