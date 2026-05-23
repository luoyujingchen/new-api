import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
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
import { Button } from '@/components/ui/button'
import { useDepartments } from './departments-provider'
import {
  type DepartmentFormValues,
  departmentFormSchema,
  DEPARTMENT_FORM_DEFAULT_VALUES,
} from '../../types'
import {
  createDepartment,
  updateDepartment,
  getAllCompanies,
  getAllDepartments,
} from '../../api'

export function DepartmentsMutateDrawer() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useDepartments()
  const isUpdate = open === 'update'
  const isCreateChild = open === 'create-child'

  const form = useForm<DepartmentFormValues>({
    resolver: zodResolver(departmentFormSchema),
    defaultValues: DEPARTMENT_FORM_DEFAULT_VALUES,
  })

  const watchCompanyId = form.watch('company_id')

  // Fetch companies for dropdown
  const { data: companiesData } = useQuery({
    queryKey: ['all-companies'],
    queryFn: async () => {
      const res = await getAllCompanies()
      return res.data || []
    },
  })

  // Fetch departments for parent dropdown (filtered by company)
  const { data: parentDepartmentsData } = useQuery({
    queryKey: ['department-parents', watchCompanyId],
    queryFn: async () => {
      if (!watchCompanyId) return []
      const res = await getAllDepartments({ company_id: watchCompanyId })
      return (res.data || []).filter(
        (d: { id: number }) => !isUpdate || !currentRow || d.id !== currentRow.id
      )
    },
    enabled: !!watchCompanyId,
  })

  useEffect(() => {
    if (open === 'create-child' && currentRow) {
      // Pre-fill company and parent from the selected department
      form.reset({
        ...DEPARTMENT_FORM_DEFAULT_VALUES,
        company_id: currentRow.company_id,
        parent_id: currentRow.id,
      })
    } else if (open && currentRow && isUpdate) {
      form.reset({
        company_id: currentRow.company_id,
        name: currentRow.name,
        parent_id: currentRow.parent_id || 0,
        description: currentRow.description || '',
        status: currentRow.status,
        sort_order: currentRow.sort_order || 0,
      })
    } else if (open === 'create') {
      form.reset({
        ...DEPARTMENT_FORM_DEFAULT_VALUES,
      })
    }
  }, [open, currentRow, form, isUpdate])

  const onSubmit = async (data: DepartmentFormValues) => {
    try {
      if (isUpdate && currentRow) {
        await updateDepartment(currentRow.id, data)
      } else {
        await createDepartment(data)
      }
      triggerRefresh()
      setOpen(null)
    } catch {
      // Error is handled by API interceptor
    }
  }

  const isSheetOpen = open === 'create' || open === 'create-child' || open === 'update'

  return (
    <Sheet
      open={isSheetOpen}
      onOpenChange={(v) => (v ? null : setOpen(null))}
    >
      <SheetContent>
        <SheetHeader>
          <SheetTitle>
            {isUpdate
              ? t('Edit Department')
              : isCreateChild
                ? t('New Sub-Department')
                : t('New Department')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update department information')
              : isCreateChild
                ? t('Create a new sub-department under') + ' ' + (currentRow?.name || '')
                : t('Create a new department')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className='space-y-4 px-4'
          >
            <FormField
              control={form.control}
              name='company_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Company')}</FormLabel>
                  <Select
                    onValueChange={(v) => {
                      field.onChange(Number(v))
                      form.setValue('parent_id', 0)
                    }}
                    value={String(field.value)}
                    disabled={isUpdate || isCreateChild}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select company')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {(companiesData || []).map(
                        (c: { id: number; name: string }) => (
                          <SelectItem key={c.id} value={String(c.id)}>
                            {c.name}
                          </SelectItem>
                        )
                      )}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Department name')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='parent_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Parent Department')}</FormLabel>
                  <Select
                    onValueChange={(v) => field.onChange(Number(v))}
                    value={String(field.value)}
                    disabled={!watchCompanyId || isCreateChild}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue
                          placeholder={
                            watchCompanyId
                              ? t('None (Top Level)')
                              : t('Select company first')
                          }
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='0'>{t('None (Top Level)')}</SelectItem>
                      {(parentDepartmentsData || []).map(
                        (d: { id: number; name: string }) => (
                          <SelectItem key={d.id} value={String(d.id)}>
                            {d.name}
                          </SelectItem>
                        )
                      )}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Description')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Status')}</FormLabel>
                  <Select
                    onValueChange={(v) => field.onChange(Number(v))}
                    value={String(field.value)}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Status')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='1'>{t('Enabled')}</SelectItem>
                      <SelectItem value='2'>{t('Disabled')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='sort_order'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sort Order')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      {...field}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <Button type='submit' className='w-full'>
              {isUpdate ? t('Save Changes') : t('Create')}
            </Button>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
