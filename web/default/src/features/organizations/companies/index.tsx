import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { CompaniesProvider, useCompanies } from './components/companies-provider'
import { CompaniesTable } from './components/companies-table'
import { CompaniesPrimaryButtons } from './components/companies-primary-buttons'
import { CompaniesMutateDrawer } from './components/companies-mutate-drawer'
import { CompaniesDeleteDialog } from './components/companies-delete-dialog'
import { OrganizationRateLimitDrawer } from '../components/organization-rate-limit-drawer'

function CompaniesContent() {
  const { open, setOpen, currentRow } = useCompanies()

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{''}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <CompaniesPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <CompaniesTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <CompaniesMutateDrawer />
      <CompaniesDeleteDialog />
      {open === 'rate-limit' && currentRow && (
        <OrganizationRateLimitDrawer
          open={true}
          onOpenChange={(v) => { if (!v) setOpen(null) }}
          orgType='company'
          orgId={currentRow.id}
          orgName={currentRow.name}
        />
      )}
    </>
  )
}

export function Companies() {
  const { t } = useTranslation()

  return (
    <CompaniesProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Companies')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <CompaniesPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <CompaniesTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <CompaniesMutateDrawer />
      <CompaniesDeleteDialog />
      <CompaniesRateLimitDrawer />
    </CompaniesProvider>
  )
}

function CompaniesRateLimitDrawer() {
  const { open, setOpen, currentRow } = useCompanies()

  if (open !== 'rate-limit' || !currentRow) return null

  return (
    <OrganizationRateLimitDrawer
      open={true}
      onOpenChange={(v) => { if (!v) setOpen(null) }}
      orgType='company'
      orgId={currentRow.id}
      orgName={currentRow.name}
    />
  )
}
