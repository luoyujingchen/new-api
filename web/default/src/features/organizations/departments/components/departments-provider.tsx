import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import { type Department, type DepartmentsDialogType } from '../../types'

type DepartmentsContextType = {
  open: DepartmentsDialogType | null
  setOpen: (str: DepartmentsDialogType | null) => void
  currentRow: Department | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Department | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const DepartmentsContext = React.createContext<DepartmentsContextType | null>(
  null
)

export function DepartmentsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<DepartmentsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Department | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <DepartmentsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </DepartmentsContext>
  )
}

export const useDepartments = () => {
  const ctx = React.useContext(DepartmentsContext)
  if (!ctx) {
    throw new Error(
      'useDepartments has to be used within <DepartmentsContext>'
    )
  }
  return ctx
}
