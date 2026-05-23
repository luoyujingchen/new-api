import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useCompanies } from './companies-provider'

export function CompaniesPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useCompanies()

  return (
    <Button onClick={() => setOpen('create')}>
      <Plus />
      {t('New Company')}
    </Button>
  )
}
