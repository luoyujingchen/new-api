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
import { getRouteApi } from '@tanstack/react-router'
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
import { MoreHorizontal, Pencil, Trash2, Gauge, FolderPlus, Power } from 'lucide-react'
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
import { deleteDepartment, updateDepartmentStatus } from '../api'
import { getDepartments } from '../api'
import { type Department } from '../types'
import { useDepartments } from './departments-provider'

const route = getRouteApi('/_authenticated/organizations/departments')

interface DepartmentsTableProps {
  onConfigureRateLimit: (department: Department) => void
}

export function DepartmentsTable({
  onConfigureRateLimit,
}: DepartmentsTableProps) {
  const { t } = useTranslation()
  const { refreshTrigger } = useDepartments()
  const search = route.useSearch()
  const {
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate: route.useNavigate(),
    pagination: {
      defaultPage: 1,
      defaultPageSize: 10,
    },
    globalFilter: { enabled: false },
  })
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const columns = useColumns({ onConfigureRateLimit })
  const companyId = search.company_id

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'departments',
      pagination.pageIndex + 1,
      pagination.pageSize,
      companyId,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getDepartments({
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        company_id: companyId,
      })
      return {
        items: result.items || [],
        total: result.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const departments = data?.items || []
  const total = data?.total || 0

  const table = useReactTable({
    data: departments,
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
      emptyTitle={t('No Departments Found')}
      emptyDescription={t(
        'No departments available. Create your first department.'
      )}
      toolbarProps={null}
      skeletonKeyPrefix='departments-skeleton'
    />
  )
}

function useColumns({
  onConfigureRateLimit,
}: {
  onConfigureRateLimit: (department: Department) => void
}) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow, triggerRefresh } = useDepartments()

  const handleToggleStatus = async (id: number, currentStatus: number, name: string) => {
    const newStatus = currentStatus === 1 ? 0 : 1
    const action = newStatus === 1 ? t('enable') : t('disable')

    const result = await updateDepartmentStatus(id, newStatus)
    if (result.success) {
      toast.success(
        t('Department "{{name}}" {{action}}d successfully', { name, action })
      )
      triggerRefresh()
    } else {
      toast.error(result.message || t('Failed to update department status'))
    }
  }

  const columns: ColumnDef<Department>[] = [
    {
      accessorKey: 'name',
      header: t('Department Name'),
      cell: ({ row }) => {
        const level = row.original.level
        const indent = (level - 1) * 24
        return (
          <div
            className="flex items-center"
            style={{ paddingLeft: `${indent}px` }}
          >
            <span className="font-medium">{row.getValue('name')}</span>
          </div>
        )
      },
    },
    {
      accessorKey: 'level',
      header: t('Level'),
      cell: ({ row }) => {
        const level = row.getValue('level') as number
        return <Badge variant='outline'>{level || 1}</Badge>
      },
    },
    {
      accessorKey: 'company',
      header: t('Company'),
      cell: ({ row }) => {
        return row.original.company?.name || '-'
      },
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
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => {
        const department = row.original
        const canCreateChild = (department.level || 1) < 4
        return (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant='ghost' size='icon' />}
            >
              <MoreHorizontal className='h-4 w-4' />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => onConfigureRateLimit(department)}>
                <Gauge className='mr-2 h-4 w-4' />
                {t('Configure RPM')}
              </DropdownMenuItem>
              {canCreateChild && (
                <DropdownMenuItem
                  onClick={() => {
                    setCurrentRow(department)
                    setOpen('create-child')
                  }}
                >
                  <FolderPlus className='mr-2 h-4 w-4' />
                  {t('Add Sub-Department')}
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => handleToggleStatus(department.id, department.status, department.name)}
              >
                <Power className='mr-2 h-4 w-4' />
                {department.status === 1 ? t('Disable') : t('Enable')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(department)
                  setOpen('update')
                }}
              >
                <Pencil className="mr-2 h-4 w-4" />
                {t('Edit')}
              </DropdownMenuItem>
              <DropdownMenuItem
                className="text-destructive"
                onClick={() => {
                  setCurrentRow(department)
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
