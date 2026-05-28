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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { deleteCompany } from '../api'
import { useCompanies } from './companies-provider'

export function CompaniesDeleteDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, setCurrentRow, triggerRefresh } = useCompanies()
  const [isDeleting, setIsDeleting] = useState(false)

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      return
    }
    setOpen(null)
    setCurrentRow(null)
  }

  const handleDelete = async () => {
    if (!currentRow) {
      return
    }
    setIsDeleting(true)
    try {
      const result = await deleteCompany(currentRow.id)
      if (result.success) {
        toast.success(t('Company deleted successfully'))
        triggerRefresh()
        handleOpenChange(false)
        return
      }
      toast.error(result.message || t('Failed to delete company'))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <Dialog open={open === 'delete'} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Delete')}</DialogTitle>
          <DialogDescription>
            {currentRow
              ? t('Are you sure you want to delete "{{name}}"?', {
                  name: currentRow.name,
                })
              : t('Are you sure you want to delete this company?')}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            variant='destructive'
            onClick={handleDelete}
            disabled={isDeleting}
          >
            {t('Delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}