import { useTranslation } from 'react-i18next'
import { type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MoreHorizontal, Pencil, Trash2, Power, FolderPlus, Gauge } from 'lucide-react'
import type { Department } from '../../types'
import { useDepartments } from './departments-provider'

export function useDepartmentsColumns(): ColumnDef<Department>[] {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useDepartments()

  return [
    {
      accessorKey: 'name',
      header: t('Name'),
      cell: ({ row }) => {
        const level = row.original.level || 1
        const indent = '  '.repeat(level - 1)
        return (
          <div className='font-medium whitespace-pre'>
            {indent}
            {level > 1 ? '└ ' : ''}
            {row.getValue('name')}
          </div>
        )
      },
    },
    {
      accessorKey: 'level',
      header: t('Level'),
      cell: ({ row }) => (
        <Badge variant='outline'>{row.original.level || 1}</Badge>
      ),
    },
    {
      accessorKey: 'company_name',
      header: t('Company'),
      cell: ({ row }) => row.original.company_name || '-',
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
        const dept = row.original
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
                  setCurrentRow(dept)
                  setOpen('update')
                }}
              >
                <Pencil />
                {t('Edit')}
              </DropdownMenuItem>
              {(dept.level || 1) < 4 && (
                <DropdownMenuItem
                  onClick={() => {
                    setCurrentRow(dept)
                    setOpen('create-child')
                  }}
                >
                  <FolderPlus />
                  {t('Add Sub-Department')}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(dept)
                  setOpen('delete')
                }}
              >
                <Trash2 />
                {t('Delete')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => {
                  const newStatus = dept.status === 1 ? 2 : 1
                  import('../../api').then(({ updateDepartmentStatus }) => {
                    updateDepartmentStatus(dept.id, newStatus)
                  })
                }}
              >
                <Power />
                {dept.status === 1 ? t('Disable') : t('Enable')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(dept)
                  setOpen('rate-limit')
                }}
              >
                <Gauge />
                {t('Configure RPM')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]
}
