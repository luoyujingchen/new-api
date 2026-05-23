import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useDepartments } from './departments-provider'

export function DepartmentsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useDepartments()

  return (
    <Button onClick={() => setOpen('create')}>
      <Plus />
      {t('New Department')}
    </Button>
  )
}
