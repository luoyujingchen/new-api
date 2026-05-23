import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { CompaniesProvider } from './components/companies-provider'
import { CompaniesTable } from './components/companies-table'
import { CompaniesPrimaryButtons } from './components/companies-primary-buttons'
import { CompaniesMutateDrawer } from './components/companies-mutate-drawer'
import { CompaniesDeleteDialog } from './components/companies-delete-dialog'

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
    </CompaniesProvider>
  )
}
