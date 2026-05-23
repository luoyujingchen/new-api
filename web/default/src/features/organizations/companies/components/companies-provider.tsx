import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import { type Company, type CompaniesDialogType } from '../../types'

type CompaniesContextType = {
  open: CompaniesDialogType | null
  setOpen: (str: CompaniesDialogType | null) => void
  currentRow: Company | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Company | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const CompaniesContext = React.createContext<CompaniesContextType | null>(null)

export function CompaniesProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<CompaniesDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Company | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <CompaniesContext
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
    </CompaniesContext>
  )
}

export const useCompanies = () => {
  const ctx = React.useContext(CompaniesContext)
  if (!ctx) {
    throw new Error('useCompanies has to be used within <CompaniesContext>')
  }
  return ctx
}
