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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { createCompany, updateCompany } from '../api'
import { type Company, type CompanyFormData, STATUS_OPTIONS } from '../types'

const companyFormSchema = z.object({
  name: z.string().min(1, 'Company name is required').max(128, 'Company name is too long'),
  code: z.string().min(1, 'Company code is required').max(32, 'Company code is too long'),
  description: z.string().max(500, 'Description is too long').optional(),
  status: z.number().int(),
  sort_order: z.number().int(),
})

type CompanyFormValues = z.infer<typeof companyFormSchema>

const DEFAULT_VALUES: CompanyFormValues = {
  name: '',
  code: '',
  description: '',
  status: 1,
  sort_order: 0,
}

interface CompaniesMutateDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Company
  onRefresh: () => void
}

export function CompaniesMutateDrawer({
  open,
  onOpenChange,
  currentRow,
  onRefresh,
}: CompaniesMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<CompanyFormValues>({
    resolver: zodResolver(companyFormSchema),
    defaultValues: DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open && isUpdate && currentRow) {
      form.reset({
        name: currentRow.name,
        code: currentRow.code,
        description: currentRow.description || '',
        status: currentRow.status,
        sort_order: currentRow.sort_order,
      })
    } else if (open && !isUpdate) {
      form.reset(DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  const onSubmit = async (data: CompanyFormValues) => {
    setIsSubmitting(true)
    try {
      const payload: CompanyFormData = {
        name: data.name,
        code: data.code,
        description: data.description,
        status: data.status,
        sort_order: data.sort_order,
      }

      const result = isUpdate
        ? await updateCompany(currentRow!.id, payload)
        : await createCompany(payload)

      if (result.success) {
        toast.success(
          isUpdate
            ? t('Company updated successfully')
            : t('Company created successfully')
        )
        onOpenChange(false)
        onRefresh()
      } else {
        toast.error(result.message || t('Operation failed'))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>
            {isUpdate ? t('Edit Company') : t('Create Company')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update company information')
              : t('Fill in the form to create a new company')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id="company-form"
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 overflow-y-auto py-4 max-h-[calc(100vh-180px)]"
          >
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Company Name')}*</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Enter company name')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="code"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Company Code')}*</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Enter company code')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      placeholder={t('Enter company description')}
                      rows={3}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="status"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Status')}</FormLabel>
                  <Select
                    onValueChange={(value) => {
                      if (value !== null) {
                        field.onChange(parseInt(value, 10))
                      }
                    }}
                    value={String(field.value)}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {STATUS_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={String(option.value)}>
                          {option.labelZh}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="sort_order"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sort Order')}</FormLabel>
                  <FormControl>
                    <Input
                      type="number"
                      {...field}
                      onChange={(e) => field.onChange(parseInt(e.target.value) || 0)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>

        <SheetFooter className="mt-4">
          <SheetClose render={<Button variant="outline" />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            type="submit"
            form="company-form"
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : isUpdate ? t('Save') : t('Create')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
