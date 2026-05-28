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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { CompaniesTable } from './companies-table'
import { CompaniesMutateDrawer } from './companies-mutate-drawer'
import { CompaniesDeleteDialog } from './companies-delete-dialog'
import { OrganizationRateLimitDrawer } from './organization-rate-limit-drawer'
import { CompaniesProvider, useCompanies } from './companies-provider'
import { type Company } from '../types'

function CompaniesContent() {
  const { t } = useTranslation()
  const { setOpen } = useCompanies()
  const [rateLimitTarget, setRateLimitTarget] = useState<Company | undefined>()

  const handleConfigureRateLimit = (company: Company) => {
    setRateLimitTarget(company)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Companies')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={() => setOpen('create')}>
            <Plus className='h-4 w-4' />
            {t('New Company')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <CompaniesTable onConfigureRateLimit={handleConfigureRateLimit} />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <CompaniesMutateDrawer />
      <CompaniesDeleteDialog />
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
                orgType: 'company',
                orgId: rateLimitTarget.id,
                orgName: rateLimitTarget.name,
              }
            : null
        }
      />
    </>
  )
}

export function CompaniesPage() {
  return (
    <CompaniesProvider>
      <CompaniesContent />
    </CompaniesProvider>
  )
}
