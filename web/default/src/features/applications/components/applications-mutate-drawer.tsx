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
import { createApplication, updateApplication } from '../api'
import { type Application, type ApplicationFormData } from '../types'

const STATUS_OPTIONS = [
  { value: 1, label: 'Enabled' },
  { value: 0, label: 'Disabled' },
] as const

type ApplicationFormValues = {
  name: string
  description?: string
  status: number
  sort_order: number
}

const DEFAULT_VALUES: ApplicationFormValues = {
  name: '',
  description: '',
  status: 1,
  sort_order: 0,
}

interface ApplicationsMutateDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Application
  onRefresh: () => void
}

export function ApplicationsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
  onRefresh,
}: ApplicationsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const [isSubmitting, setIsSubmitting] = useState(false)
  const applicationFormSchema = z.object({
    name: z
      .string()
      .min(1, t('Application name is required'))
      .max(128, t('Application name is too long')),
    description: z
      .string()
      .max(500, t('Description is too long'))
      .optional(),
    status: z.number().int(),
    sort_order: z.number().int(),
  })

  const form = useForm<ApplicationFormValues>({
    resolver: zodResolver(applicationFormSchema),
    defaultValues: DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open && isUpdate && currentRow) {
      form.reset({
        name: currentRow.name,
        description: currentRow.description || '',
        status: currentRow.status,
        sort_order: currentRow.sort_order,
      })
    } else if (open && !isUpdate) {
      form.reset(DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  const onSubmit = async (data: ApplicationFormValues) => {
    setIsSubmitting(true)
    try {
      const payload: ApplicationFormData = {
        name: data.name,
        description: data.description,
        status: data.status,
        sort_order: data.sort_order,
      }

      const result = isUpdate
        ? await updateApplication(currentRow!.id, payload)
        : await createApplication(payload)

      if (result.success) {
        toast.success(
          isUpdate
            ? t('Application updated successfully')
            : t('Application created successfully')
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
            {isUpdate ? t('Edit Application') : t('Create Application')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update application information')
              : t('Fill in the form to create a new application')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id="application-form"
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 overflow-y-auto py-4 max-h-[calc(100vh-180px)]"
          >
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Application Name')}*</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Enter application name')} />
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
                      placeholder={t('Enter application description')}
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
                          {t(option.label)}
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
            form="application-form"
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : isUpdate ? t('Save') : t('Create')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
