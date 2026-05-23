import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
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
import { useCompanies } from './companies-provider'
import {
  type CompanyFormValues,
  companyFormSchema,
  COMPANY_FORM_DEFAULT_VALUES,
} from '../../types'
import { createCompany, updateCompany } from '../../api'

export function CompaniesMutateDrawer() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useCompanies()
  const isUpdate = open === 'update'

  const form = useForm<CompanyFormValues>({
    resolver: zodResolver(companyFormSchema),
    defaultValues: COMPANY_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (open && currentRow) {
      form.reset({
        name: currentRow.name,
        code: currentRow.code,
        description: currentRow.description || '',
        status: currentRow.status,
        sort_order: currentRow.sort_order || 0,
      })
    } else if (open === 'create') {
      form.reset(COMPANY_FORM_DEFAULT_VALUES)
    }
  }, [open, currentRow, form])

  const onSubmit = async (data: CompanyFormValues) => {
    try {
      if (isUpdate && currentRow) {
        await updateCompany(currentRow.id, data)
      } else {
        await createCompany(data)
      }
      triggerRefresh()
      setOpen(null)
    } catch {
      // Error is handled by API interceptor
    }
  }

  return (
    <Sheet
      open={open === 'create' || open === 'update'}
      onOpenChange={(v) => (v ? null : setOpen(null))}
    >
      <SheetContent>
        <SheetHeader>
          <SheetTitle>
            {isUpdate ? t('Edit Company') : t('New Company')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update company information')
              : t('Create a new company')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className='space-y-4 px-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Company name')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='code'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Code')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Company code')} />
                  </FormControl>
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
