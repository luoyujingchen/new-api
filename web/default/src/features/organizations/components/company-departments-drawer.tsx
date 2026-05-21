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
import { useQuery } from '@tanstack/react-query'
import { X, Plus, Building2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { getDepartmentTree } from '../api'
import { DepartmentsMutateDrawer } from './departments-mutate-drawer'
import { type Company, type DepartmentTreeNode } from '../types'

type CompanyDepartmentsDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  company: Company | null
  onRefresh: () => void
}

export function CompanyDepartmentsDrawer({
  open,
  onOpenChange,
  company,
  onRefresh,
}: CompanyDepartmentsDrawerProps) {
  const { t } = useTranslation()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingDept, setEditingDept] = useState<DepartmentTreeNode | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const { data: treeData, isLoading } = useQuery({
    queryKey: ['department-tree', company?.id, refreshTrigger],
    queryFn: () => getDepartmentTree(company!.id),
    enabled: !!company && open,
  })

  const tree = treeData?.data ?? []

  const handleCreateDept = () => {
    setEditingDept(null)
    setIsCreateOpen(true)
  }

  const handleEditDept = (dept: DepartmentTreeNode) => {
    setEditingDept(dept)
    setIsCreateOpen(true)
  }

  const handleDeptSaved = () => {
    setIsCreateOpen(false)
    setEditingDept(null)
    setRefreshTrigger((prev) => prev + 1)
    onRefresh()
  }

  const renderDepartmentNode = (dept: DepartmentTreeNode, level = 0) => {
    const indent = level * 20
    return (
      <div key={dept.id}>
        <div
          className="flex items-center justify-between py-2 px-3 hover:bg-muted/50 rounded-md cursor-pointer group"
          style={{ paddingLeft: `${indent + 12}px` }}
          onClick={() => handleEditDept(dept)}
        >
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground text-sm">
              {dept.level > 1 && '└'}
            </span>
            <span className="text-sm font-medium">{dept.name}</span>
            {dept.status !== 1 && (
              <span className="text-xs text-muted-foreground">({t('Disabled')})</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {dept.children?.length || 0} {t('Sub')}
            </span>
          </div>
        </div>
        {dept.children && dept.children.length > 0 && (
          <div>
            {dept.children.map((child) => renderDepartmentNode(child, level + 1))}
          </div>
        )}
      </div>
    )
  }

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className="flex w-full flex-col sm:max-w-[600px]">
          <SheetHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Building2 className="h-5 w-5" />
                <SheetTitle>{company?.name}</SheetTitle>
              </div>
              <SheetClose render={<Button variant='ghost' size='sm' />}>
                <X className='h-4 w-4' />
                <span className='ml-1'>{t('Close')}</span>
              </SheetClose>
            </div>
            <p className="text-sm text-muted-foreground">
              {t('Department Management')}
            </p>
          </SheetHeader>

          <div className="flex-1 overflow-hidden py-4">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-sm font-medium">{t('Department Tree')}</h3>
              <Button size="sm" onClick={handleCreateDept}>
                <Plus className="mr-1 h-3 w-3" />
                {t('New Department')}
              </Button>
            </div>

            {isLoading ? (
              <div className="text-center text-sm text-muted-foreground">
                {t('Loading...')}
              </div>
            ) : tree.length > 0 ? (
              <ScrollArea className="h-[calc(100vh-250px)]">
                <div className="space-y-1 pr-4">
                  {tree.map((dept) => renderDepartmentNode(dept))}
                </div>
              </ScrollArea>
            ) : (
              <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">
                {t('No departments yet. Click "New Department" to create one.')}
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>

      {/* Create/Edit Department Drawer */}
      <DepartmentsMutateDrawer
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        defaultCompanyId={company?.id}
        currentRow={editingDept ?? undefined}
        onRefresh={handleDeptSaved}
      />
    </>
  )
}
