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
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DepartmentsTable } from './departments-table'
import { DepartmentsMutateDrawer } from './departments-mutate-drawer'
import { getDepartments, getCompany } from '../api'
import { type Department } from '../types'

interface DepartmentsSearch {
  company_id?: number
}

export function DepartmentsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ from: '/_authenticated/organizations/departments' })

  const companyId = (search as DepartmentsSearch).company_id

  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [currentRow, setCurrentRow] = useState<Department | undefined>()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  // Fetch company info if company_id is in search params
  const { data: companyResponse } = useQuery({
    queryKey: ['company', companyId],
    queryFn: () => getCompany(companyId!),
    enabled: !!companyId,
  })
  const company = companyResponse?.data

  const { data, isLoading } = useQuery({
    queryKey: ['departments', page, pageSize, refreshTrigger, companyId],
    queryFn: () => getDepartments({
      page,
      page_size: pageSize,
      company_id: companyId,
    }),
  })

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
  }

  const handleEdit = (department: Department) => {
    setCurrentRow(department)
  }

  const handleRefresh = () => {
    setRefreshTrigger((prev) => prev + 1)
  }

  const handleClearCompanyFilter = () => {
    navigate({ to: '/organizations/departments', search: { company_id: undefined } })
  }

  if (isLoading) {
    return <div className="p-4">Loading...</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold">Departments</h1>
            {company && (
              <>
                <span className="text-muted-foreground">/</span>
                <span className="text-2xl font-medium text-muted-foreground">
                  {company.name}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 w-6 p-0"
                  onClick={handleClearCompanyFilter}
                >
                  <X className="h-4 w-4" />
                </Button>
              </>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            {company
              ? t('Manage departments for {{companyName}}', { companyName: company.name })
              : t('Manage department hierarchy')}
          </p>
        </div>
        <Button onClick={() => setIsCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          New Department
        </Button>
      </div>

      <DepartmentsTable
        data={data?.items || []}
        total={data?.total || 0}
        page={page}
        pageSize={pageSize}
        onPageChange={handlePageChange}
        onEdit={handleEdit}
        onRefresh={handleRefresh}
      />

      {/* Create Drawer */}
      <DepartmentsMutateDrawer
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        onRefresh={handleRefresh}
        defaultCompanyId={companyId}
      />

      {/* Edit Drawer */}
      <DepartmentsMutateDrawer
        open={!!currentRow}
        onOpenChange={(open) => {
          if (!open) setCurrentRow(undefined)
        }}
        currentRow={currentRow}
        onRefresh={handleRefresh}
      />
    </div>
  )
}
