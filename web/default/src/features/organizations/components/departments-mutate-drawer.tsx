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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
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
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import {
  createDepartment,
  getAllCompanies,
  getAllDepartments,
  updateDepartment,
} from '../api'
import { type Department, type DepartmentFormData, STATUS_OPTIONS } from '../types'
import { useDepartments } from './departments-provider'

const departmentFormSchema = z.object({
  company_id: z.number().int().positive('Company is required'),
  name: z.string().min(1, 'Department name is required').max(128, 'Department name is too long'),
  parent_id: z.number().int().nullable().optional(),
  description: z.string().max(500, 'Description is too long').optional(),
  status: z.number().int(),
  sort_order: z.number().int(),
})

type DepartmentFormValues = z.infer<typeof departmentFormSchema>

const DEFAULT_VALUES: DepartmentFormValues = {
  company_id: 0,
  name: '',
  parent_id: null,
  description: '',
  status: 1,
  sort_order: 0,
}

interface DepartmentsMutateDrawerProps {
  defaultCompanyId?: number
}

const route = getRouteApi('/_authenticated/organizations/departments')

export function DepartmentsMutateDrawer(_props: DepartmentsMutateDrawerProps) {
  const { t } = useTranslation()
  const search = route.useSearch()
  const { open, setOpen, currentRow, setCurrentRow, triggerRefresh } = useDepartments()
  const isCreateChild = open === 'create-child'
  const isUpdate = open === 'update'
  const filteredCompanyId = search.company_id
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [selectedCompanyId, setSelectedCompanyId] = useState<number | null>(
    filteredCompanyId || null
  )

  const sheetOpen = open === 'create' || open === 'create-child' || open === 'update'

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      return
    }
    setOpen(null)
    setCurrentRow(null)
  }

  const { data: companiesData } = useQuery({
    queryKey: ['companies', 'all'],
    queryFn: () => getAllCompanies(1), // Only enabled companies
    staleTime: 5 * 60 * 1000,
  })

  const { data: departmentsData } = useQuery({
    queryKey: ['departments', 'all', selectedCompanyId],
    queryFn: () => getAllDepartments(selectedCompanyId || undefined, 1),
    enabled: !!selectedCompanyId,
    staleTime: 2 * 60 * 1000,
  })

  const companies = companiesData?.data || []
  const departments = departmentsData?.data || []

  const form = useForm<DepartmentFormValues>({
    resolver: zodResolver(departmentFormSchema),
    defaultValues: DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open === 'create-child' && currentRow) {
      setSelectedCompanyId(currentRow.company_id)
      form.reset({
        ...DEFAULT_VALUES,
        company_id: currentRow.company_id,
        parent_id: currentRow.id,
      })
    } else if (sheetOpen && isUpdate && currentRow) {
      setSelectedCompanyId(currentRow.company_id)
      form.reset({
        company_id: currentRow.company_id,
        name: currentRow.name,
        parent_id: currentRow.parent_id ?? null,
        description: currentRow.description || '',
        status: currentRow.status,
        sort_order: currentRow.sort_order,
      })
    } else if (sheetOpen && !isUpdate) {
      if (filteredCompanyId) {
        setSelectedCompanyId(filteredCompanyId)
        form.reset({
          ...DEFAULT_VALUES,
          company_id: filteredCompanyId,
        })
      } else {
        setSelectedCompanyId(null)
        form.reset(DEFAULT_VALUES)
      }
    }
  }, [sheetOpen, open, isUpdate, currentRow, filteredCompanyId, form])

  const handleCompanyChange = (companyId: number) => {
    setSelectedCompanyId(companyId)
    form.setValue('parent_id', null)
  }

  const availableParentDepartments = departments.filter(
    (dept) => !isUpdate || dept.id !== currentRow!.id
  )

  const onSubmit = async (data: DepartmentFormValues) => {
    setIsSubmitting(true)
    try {
      const createPayload: DepartmentFormData = {
        company_id: data.company_id,
        name: data.name,
        parent_id: data.parent_id ?? null,
        description: data.description,
        status: data.status,
        sort_order: data.sort_order,
      }
      const result = isUpdate
        ? await updateDepartment(currentRow!.id, {
            name: data.name,
            parent_id: data.parent_id ?? null,
            description: data.description,
            status: data.status,
            sort_order: data.sort_order,
          })
        : await createDepartment(createPayload)

      if (result.success) {
        toast.success(
          isUpdate
            ? t('Department updated successfully')
            : t('Department created successfully')
        )
        handleOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(result.message || t('Operation failed'))
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={sheetOpen} onOpenChange={handleOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>
            {isUpdate
              ? t('Edit Department')
              : isCreateChild
                ? t('New Sub-Department')
                : t('Create Department')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update department information')
              : isCreateChild && currentRow
                ? `${t('Create a new department')} ${currentRow.name}`
                : t('Fill in the form to create a new department')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id="department-form"
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 overflow-y-auto py-4 max-h-[calc(100vh-180px)]"
          >
            <FormField
              control={form.control}
              name="company_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Company')}*</FormLabel>
                  <Select
                    onValueChange={(value) => {
                      if (value !== null) {
                        const companyId = parseInt(value, 10)
                        field.onChange(companyId)
                        handleCompanyChange(companyId)
                      }
                    }}
                    value={field.value ? String(field.value) : ''}
                    disabled={isUpdate || isCreateChild}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select a company')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {companies.map((company) => (
                        <SelectItem key={company.id} value={String(company.id)}>
                          {company.name}
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
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Department Name')}*</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Enter department name')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="parent_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Parent Department')}</FormLabel>
                  <Select
                    onValueChange={(value) =>
                      field.onChange(value ? parseInt(value, 10) : null)
                    }
                    value={field.value ? String(field.value) : ''}
                    disabled={!selectedCompanyId || isCreateChild}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('No parent (root level)')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="">{t('No parent (root level)')}</SelectItem>
                      {availableParentDepartments.map((dept) => (
                        <SelectItem key={dept.id} value={String(dept.id)}>
                          {'\u00A0'.repeat(Math.max(0, (dept.level - 1) * 2))}
                          {dept.name}
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
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      placeholder={t('Enter department description')}
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

        <div className='mt-4 flex items-center justify-end gap-2'>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button type='submit' form='department-form' disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : isUpdate ? t('Save') : t('Create')}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
