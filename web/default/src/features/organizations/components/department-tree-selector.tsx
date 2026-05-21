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
import { ChevronDown, ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { type DepartmentTreeNode } from '../types'

interface DepartmentTreeSelectorProps {
  tree: DepartmentTreeNode[]
  value?: number | null
  onChange: (value: number | null) => void
  placeholder?: string
  disabled?: boolean
  excludeId?: number // Exclude this department (useful when moving a department)
}

export function DepartmentTreeSelector({
  tree,
  value,
  onChange,
  placeholder = 'Select a department',
  disabled = false,
  excludeId,
}: DepartmentTreeSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [expandedNodes, setExpandedNodes] = useState<Set<number>>(new Set())

  const selectedDept = findDepartmentById(tree, value)

  const toggleExpand = (id: number) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const handleSelect = (id: number) => {
    onChange(id)
    setIsOpen(false)
  }

  const handleClear = () => {
    onChange(null)
  }

  return (
    <div className="relative">
      <Button
        type="button"
        variant="outline"
        className={cn(
          'w-full justify-between text-left font-normal',
          !selectedDept && 'text-muted-foreground'
        )}
        onClick={() => setIsOpen(!isOpen)}
        disabled={disabled}
      >
        <span className="truncate">
          {selectedDept ? selectedDept.name : placeholder}
        </span>
        <ChevronDown
          className={cn(
            'h-4 w-4 shrink-0 opacity-50 transition-transform',
            isOpen && 'transform rotate-180'
          )}
        />
      </Button>

      {selectedDept && (
        <button
          type="button"
          className="absolute right-8 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          onClick={(e) => {
            e.stopPropagation()
            handleClear()
          }}
        >
          ×
        </button>
      )}

      {isOpen && (
        <div className="absolute z-50 w-full mt-1 bg-background border rounded-md shadow-lg max-h-[300px] overflow-y-auto">
          <div className="p-2">
            <button
              type="button"
              className={cn(
                'w-full text-left px-2 py-1.5 rounded-sm hover:bg-accent text-sm',
                !value && 'bg-accent'
              )}
              onClick={() => handleSelect(0 as any)}
            >
              {placeholder}
            </button>
            {renderTree(
              tree,
              value,
              handleSelect,
              expandedNodes,
              toggleExpand,
              0,
              excludeId
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function renderTree(
  nodes: DepartmentTreeNode[],
  value: number | null | undefined,
  onSelect: (id: number) => void,
  expandedNodes: Set<number>,
  onToggleExpand: (id: number) => void,
  level: number,
  excludeId?: number
) {
  return nodes.map((node) => {
    const isExcluded = node.id === excludeId
    const hasChildren = node.children && node.children.length > 0
    const isExpanded = expandedNodes.has(node.id)
    const isSelected = value === node.id

    return (
      <div key={node.id}>
        <div
          className={cn(
            'flex items-center gap-1 px-2 py-1.5 rounded-sm hover:bg-accent text-sm cursor-pointer',
            isSelected && 'bg-accent',
            isExcluded && 'opacity-50 cursor-not-allowed'
          )}
          style={{ paddingLeft: `${level * 16 + 8}px` }}
          onClick={() => !isExcluded && onSelect(node.id)}
        >
          {hasChildren && (
            <button
              type="button"
              className="shrink-0"
              onClick={(e) => {
                e.stopPropagation()
                onToggleExpand(node.id)
              }}
            >
              {isExpanded ? (
                <ChevronDown className="h-3 w-3" />
              ) : (
                <ChevronRight className="h-3 w-3" />
              )}
            </button>
          )}
          {!hasChildren && <span className="w-4 shrink-0" />}
          <span className="flex-1 truncate">{node.name}</span>
          <Badge variant="outline" className="ml-auto text-xs">
            L{node.level}
          </Badge>
        </div>
        {hasChildren && isExpanded && (
          <div>
            {renderTree(
              node.children!,
              value,
              onSelect,
              expandedNodes,
              onToggleExpand,
              level + 1,
              excludeId
            )}
          </div>
        )}
      </div>
    )
  })
}

function findDepartmentById(
  nodes: DepartmentTreeNode[],
  id: number | null | undefined
): DepartmentTreeNode | null {
  if (!id) return null

  for (const node of nodes) {
    if (node.id === id) return node
    if (node.children) {
      const found = findDepartmentById(node.children, id)
      if (found) return found
    }
  }
  return null
}

// Compact version for use in forms
interface DepartmentTreeSelectorCompactProps {
  tree: DepartmentTreeNode[]
  value?: number | null
  onChange: (value: number | null) => void
  placeholder?: string
  disabled?: boolean
  excludeId?: number
}

export function DepartmentTreeSelectorCompact({
  tree,
  value,
  onChange,
  disabled = false,
  excludeId,
}: DepartmentTreeSelectorCompactProps) {
  const selectedDept = findDepartmentById(tree, value)

  return (
    <div className="space-y-2">
      <label className="text-sm font-medium">Parent Department</label>
      <div className="border rounded-md p-2 max-h-[200px] overflow-y-auto">
        <button
          type="button"
          disabled={disabled}
          className={cn(
            'w-full text-left px-2 py-1 rounded-sm hover:bg-accent text-sm',
            !value && 'bg-accent'
          )}
          onClick={() => onChange(null)}
        >
          No parent (root level)
        </button>
        {renderTreeCompact(
          tree,
          value,
          onChange,
          disabled,
          0,
          excludeId
        )}
      </div>
      {selectedDept && (
        <p className="text-xs text-muted-foreground">
          Selected: {selectedDept.name} (Level {selectedDept.level})
        </p>
      )}
    </div>
  )
}

function renderTreeCompact(
  nodes: DepartmentTreeNode[],
  value: number | null | undefined,
  onSelect: (value: number | null) => void,
  disabled: boolean,
  level: number,
  excludeId?: number
) {
  return nodes.map((node) => {
    const isExcluded = node.id === excludeId
    const isSelected = value === node.id
    const hasChildren = node.children && node.children.length > 0

    return (
      <div key={node.id}>
        <div
          className={cn(
            'px-2 py-1 rounded hover:bg-accent text-sm cursor-pointer',
            isSelected && 'bg-accent',
            isExcluded && 'opacity-50 cursor-not-allowed'
          )}
          style={{ paddingLeft: `${level * 16 + 8}px` }}
          onClick={() => !isExcluded && !disabled && onSelect(node.id)}
        >
          {node.name}
        </div>
        {hasChildren && (
          <div>
            {renderTreeCompact(
              node.children!,
              value,
              onSelect,
              disabled,
              level + 1,
              excludeId
            )}
          </div>
        )}
      </div>
    )
  })
}
