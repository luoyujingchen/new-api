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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import {
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { MoreHorizontal, Pencil, Trash2, FolderTree, Gauge, Power } from 'lucide-react'
import { toast } from 'sonner'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTablePage } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Badge } from '@/components/ui/badge'
import { getCompanies, updateCompanyStatus } from '../api'
import { type Company } from '../types'
import { useCompanies } from './companies-provider'

const route = getRouteApi('/_authenticated/organizations/companies')

interface CompaniesTableProps {
  onConfigureRateLimit: (company: Company) => void
}

export function CompaniesTable({
  onConfigureRateLimit,
}: CompaniesTableProps) {
  const { t } = useTranslation()
  const { refreshTrigger } = useCompanies()
  const {
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: {
      defaultPage: 1,
      defaultPageSize: 10,
    },
    globalFilter: { enabled: false },
  })
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const columns = useColumns({
    onConfigureRateLimit,
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'companies',
      pagination.pageIndex + 1,
      pagination.pageSize,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getCompanies({
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
      return {
        items: result.items || [],
        total: result.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const companies = data?.items || []
  const total = data?.total || 0

  const table = useReactTable({
    data: companies,
    columns,
    pageCount: Math.ceil(total / Math.max(1, pagination.pageSize)),
    state: {
      sorting,
      columnVisibility,
      pagination,
    },
    onPaginationChange,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualPagination: true,
  })

  useEffect(() => {
    ensurePageInRange(table.getPageCount())
  }, [ensurePageInRange, table, total])

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Companies Found')}
      emptyDescription={t(
        'No companies available. Create your first company.'
      )}
      toolbarProps={null}
      skeletonKeyPrefix='companies-skeleton'
    />
  )
}

function useColumns({
  onConfigureRateLimit,
}: {
  onConfigureRateLimit: (company: Company) => void
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setOpen, setCurrentRow, triggerRefresh } = useCompanies()

  const handleToggleStatus = async (id: number, currentStatus: number, name: string) => {
    const newStatus = currentStatus === 1 ? 0 : 1
    const action = newStatus === 1 ? t('enable') : t('disable')

    const result = await updateCompanyStatus(id, newStatus)
    if (result.success) {
      toast.success(
        t('Company "{{name}}" {{action}}d successfully', { name, action })
      )
      triggerRefresh()
    } else {
      toast.error(result.message || t('Failed to update company status'))
    }
  }

  const columns: ColumnDef<Company>[] = [
    {
      accessorKey: 'name',
      header: t('Company Name'),
      cell: ({ row }) => <span className="font-medium">{row.getValue('name')}</span>,
    },
    {
      accessorKey: 'code',
      header: t('Code'),
      cell: ({ row }) => (
        <span className="text-muted-foreground">{row.getValue('code')}</span>
      ),
    },
    {
      accessorKey: 'description',
      header: t('Description'),
      cell: ({ row }) => {
        const desc = row.getValue('description') as string
        return desc ? (
          <span className="text-sm text-muted-foreground truncate max-w-[200px] block">
            {desc}
          </span>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      accessorKey: 'department_count',
      header: t('Departments'),
      cell: ({ row }) => <span>{(row.getValue('department_count') as number) ?? 0}</span>,
    },
    {
      accessorKey: 'user_count',
      header: t('Users'),
      cell: ({ row }) => {
        const count = row.getValue('user_count') as number
        return <span>{count ?? 0}</span>
      },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as number
        return (
          <Badge
            variant='secondary'
            className={
              status === 1
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
                : undefined
            }
          >
            {status === 1 ? t('Enabled') : t('Disabled')}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'queue_priority',
      header: t('Queue Priority'),
      cell: ({ row }) => <span>{row.getValue('queue_priority') as number}</span>,
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => {
        const company = row.original
        return (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant='ghost' size='icon' />}
            >
              <MoreHorizontal className='h-4 w-4' />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onClick={() => {
                  navigate({
                    to: '/organizations/departments',
                    search: { company_id: company.id },
                  })
                }}
              >
                <FolderTree className="mr-2 h-4 w-4" />
                {t('View Departments')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onConfigureRateLimit(company)}>
                <Gauge className='mr-2 h-4 w-4' />
                {t('Configure RPM')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => handleToggleStatus(company.id, company.status, company.name)}
              >
                <Power className='mr-2 h-4 w-4' />
                {company.status === 1 ? t('Disable') : t('Enable')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(company)
                  setOpen('update')
                }}
              >
                <Pencil className="mr-2 h-4 w-4" />
                {t('Edit')}
              </DropdownMenuItem>
              <DropdownMenuItem
                className="text-destructive"
                onClick={() => {
                  setCurrentRow(company)
                  setOpen('delete')
                }}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                {t('Delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]

  return columns
}
