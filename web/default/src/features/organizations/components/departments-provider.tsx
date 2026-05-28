/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import { type Department } from '../types'

type DepartmentsDialogType = 'create' | 'create-child' | 'update' | 'delete'

type DepartmentsContextType = {
  open: DepartmentsDialogType | null
  setOpen: (value: DepartmentsDialogType | null) => void
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

  const triggerRefresh = () => {
    setRefreshTrigger((previous) => previous + 1)
  }

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

export function useDepartments() {
  const context = React.useContext(DepartmentsContext)
  if (!context) {
    throw new Error(
      'useDepartments has to be used within <DepartmentsProvider>'
    )
  }
  return context
}