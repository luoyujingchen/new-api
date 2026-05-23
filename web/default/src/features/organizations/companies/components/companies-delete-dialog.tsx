import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { useCompanies } from './companies-provider'
import { deleteCompany } from '../../api'

export function CompaniesDeleteDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useCompanies()

  const handleDelete = async () => {
    if (!currentRow) return
    try {
      await deleteCompany(currentRow.id)
      triggerRefresh()
      setOpen(null)
    } catch {
      // Error is handled by API interceptor
    }
  }

  return (
    <Dialog open={open === 'delete'} onOpenChange={(v) => (v ? null : setOpen(null))}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Delete Company')}</DialogTitle>
          <DialogDescription>
            {t(
              'Are you sure you want to delete this company? This action cannot be undone.'
            )}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant='outline' onClick={() => setOpen(null)}>
            {t('Cancel')}
          </Button>
          <Button variant='destructive' onClick={handleDelete}>
            {t('Delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
