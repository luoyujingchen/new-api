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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ApplicationsTable } from './applications-table'
import { ApplicationsMutateDrawer } from './applications-mutate-drawer'
import { getApplications } from '../api'
import { type Application } from '../types'

export function ApplicationsPage() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [currentRow, setCurrentRow] = useState<Application | undefined>()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const { data, isLoading } = useQuery({
    queryKey: ['applications', page, pageSize, refreshTrigger],
    queryFn: () => getApplications({ page, page_size: pageSize }),
  })

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
  }

  const handleEdit = (application: Application) => {
    setCurrentRow(application)
  }

  const handleRefresh = () => {
    setRefreshTrigger((prev) => prev + 1)
  }

  if (isLoading) {
    return <div className="p-4">Loading...</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('Applications')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('Manage applications for API key association')}
          </p>
        </div>
        <Button onClick={() => setIsCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t('New Application')}
        </Button>
      </div>

      <ApplicationsTable
        data={data?.items || []}
        total={data?.total || 0}
        page={page}
        pageSize={pageSize}
        onPageChange={handlePageChange}
        onEdit={handleEdit}
        onRefresh={handleRefresh}
      />

      {/* Create Drawer */}
      <ApplicationsMutateDrawer
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        onRefresh={handleRefresh}
      />

      {/* Edit Drawer */}
      <ApplicationsMutateDrawer
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
