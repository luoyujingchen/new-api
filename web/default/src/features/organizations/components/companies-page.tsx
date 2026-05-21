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
import { useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CompaniesTable } from './companies-table'
import { CompaniesMutateDrawer } from './companies-mutate-drawer'
import { getCompanies } from '../api'
import { type Company } from '../types'

export function CompaniesPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [currentRow, setCurrentRow] = useState<Company | undefined>()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const { data, isLoading } = useQuery({
    queryKey: ['companies', page, pageSize, refreshTrigger],
    queryFn: () => getCompanies({ page, page_size: pageSize }),
  })

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
  }

  const handleEdit = (company: Company) => {
    setCurrentRow(company)
  }

  const handleRefresh = () => {
    setRefreshTrigger((prev) => prev + 1)
  }

  const handleViewDepartments = (company: Company) => {
    navigate({
      to: '/organizations/departments',
      search: { company_id: company.id },
    })
  }

  if (isLoading) {
    return <div className="p-4">Loading...</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Companies</h1>
          <p className="text-sm text-muted-foreground">
            Manage sub-business companies
          </p>
        </div>
        <Button onClick={() => setIsCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          New Company
        </Button>
      </div>

      <CompaniesTable
        data={data?.items || []}
        total={data?.total || 0}
        page={page}
        pageSize={pageSize}
        onPageChange={handlePageChange}
        onEdit={handleEdit}
        onViewDepartments={handleViewDepartments}
        onRefresh={handleRefresh}
      />

      {/* Create Drawer */}
      <CompaniesMutateDrawer
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        onRefresh={handleRefresh}
      />

      {/* Edit Drawer */}
      <CompaniesMutateDrawer
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
