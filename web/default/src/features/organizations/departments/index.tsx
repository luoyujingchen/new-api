import { useTranslation } from 'react-i18next'
import { useSearch, useNavigate } from '@tanstack/react-router'
import { X } from 'lucide-react'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { DepartmentsProvider } from './components/departments-provider'
import { DepartmentsTable } from './components/departments-table'
import { DepartmentsPrimaryButtons } from './components/departments-primary-buttons'
import { DepartmentsMutateDrawer } from './components/departments-mutate-drawer'
import { DepartmentsDeleteDialog } from './components/departments-delete-dialog'

export function Departments() {
  const { t } = useTranslation()
  const search = useSearch({ strict: false }) as { company_id?: number }
  const navigate = useNavigate()
  const companyId = search.company_id

  const clearCompanyFilter = () => {
    navigate({ to: '/organizations/departments' })
  }

  return (
    <DepartmentsProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Departments')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <DepartmentsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <DepartmentsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <DepartmentsMutateDrawer />
      <DepartmentsDeleteDialog />
    </DepartmentsProvider>
  )
}
