import { useTranslation } from 'react-i18next'
import { type ColumnDef } from '@tanstack/react-table'
import { useNavigate } from '@tanstack/react-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MoreHorizontal, Pencil, Trash2, Building2, Power, Gauge } from 'lucide-react'
import type { Company } from '../../types'
import { useCompanies } from './companies-provider'

export function useCompaniesColumns(): ColumnDef<Company>[] {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useCompanies()
  const navigate = useNavigate()

  return [
    {
      accessorKey: 'name',
      header: t('Name'),
      cell: ({ row }) => (
        <div className='font-medium'>{row.getValue('name')}</div>
      ),
    },
    {
      accessorKey: 'code',
      header: t('Code'),
    },
    {
      accessorKey: 'description',
      header: t('Description'),
      cell: ({ row }) => (
        <div className='max-w-[200px] truncate'>
          {row.getValue('description') || '-'}
        </div>
      ),
    },
    {
      accessorKey: 'department_count',
      header: t('Departments'),
      cell: ({ row }) => (
        <Badge variant='secondary'>
          {row.original.department_count ?? 0}
        </Badge>
      ),
    },
    {
      accessorKey: 'user_count',
      header: t('Users'),
      cell: ({ row }) => (
        <Badge variant='secondary'>{row.original.user_count ?? 0}</Badge>
      ),
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as number
        return (
          <Badge variant={status === 1 ? 'default' : 'destructive'}>
            {status === 1 ? t('Enabled') : t('Disabled')}
          </Badge>
        )
      },
    },
    {
      id: 'actions',
      header: () => null,
      cell: ({ row }) => {
        const company = row.original
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant='ghost' className='h-8 w-8 p-0'>
                <MoreHorizontal className='h-4 w-4' />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end'>
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(company)
                  setOpen('update')
                }}
              >
                <Pencil />
                {t('Edit')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(company)
                  setOpen('delete')
                }}
              >
                <Trash2 />
                {t('Delete')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => {
                  const newStatus = company.status === 1 ? 2 : 1
                  import('../../api').then(({ updateCompanyStatus }) => {
                    updateCompanyStatus(company.id, newStatus)
                  })
                }}
              >
                <Power />
                {company.status === 1 ? t('Disable') : t('Enable')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(company)
                  setOpen('rate-limit')
                }}
              >
                <Gauge />
                {t('Configure RPM')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  navigate({ to: '/organizations/departments', search: { company_id: company.id } })
                }}
              >
                <Building2 />
                {t('View Departments')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]
}
