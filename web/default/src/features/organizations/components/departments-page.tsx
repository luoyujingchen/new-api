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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Plus, X } from 'lucide-react'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DepartmentsTable } from './departments-table'
import { DepartmentsMutateDrawer } from './departments-mutate-drawer'
import { DepartmentsDeleteDialog } from './departments-delete-dialog'
import { OrganizationRateLimitDrawer } from './organization-rate-limit-drawer'
import { DepartmentsProvider, useDepartments } from './departments-provider'
import { getCompany } from '../api'
import { type Department } from '../types'

const route = getRouteApi('/_authenticated/organizations/departments')

function DepartmentsContent() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const { setOpen } = useDepartments()
  const companyId = search.company_id
  const [rateLimitTarget, setRateLimitTarget] = useState<Department | undefined>()

  const { data: companyResponse } = useQuery({
    queryKey: ['company', companyId],
    queryFn: () => getCompany(companyId!),
    enabled: !!companyId,
  })
  const company = companyResponse?.data

  const handleConfigureRateLimit = (department: Department) => {
    setRateLimitTarget(department)
  }

  const handleClearCompanyFilter = () => {
    navigate({
      search: (previous) => ({
        ...previous,
        page: 1,
        company_id: undefined,
      }),
    })
  }

  return (
    <>
      <SectionPageLayout>
        {company && (
          <SectionPageLayout.Breadcrumb>
            <div className='flex items-center gap-2'>
              <Badge variant='secondary'>{company.name}</Badge>
              <Button
                variant='ghost'
                size='icon-sm'
                onClick={handleClearCompanyFilter}
              >
                <X className='h-4 w-4' />
              </Button>
            </div>
          </SectionPageLayout.Breadcrumb>
        )}
        <SectionPageLayout.Title>{t('Departments')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={() => setOpen('create')}>
            <Plus className='h-4 w-4' />
            {t('New Department')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <DepartmentsTable onConfigureRateLimit={handleConfigureRateLimit} />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <DepartmentsMutateDrawer />
      <DepartmentsDeleteDialog />
      <OrganizationRateLimitDrawer
        open={!!rateLimitTarget}
        onOpenChange={(open) => {
          if (!open) {
            setRateLimitTarget(undefined)
          }
        }}
        target={
          rateLimitTarget
            ? {
                orgType: 'department',
                orgId: rateLimitTarget.id,
                orgName: rateLimitTarget.name,
              }
            : null
        }
      />
    </>
  )
}

export function DepartmentsPage() {
  return (
    <DepartmentsProvider>
      <DepartmentsContent />
    </DepartmentsProvider>
  )
}
