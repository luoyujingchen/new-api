import { z } from 'zod'

// ============================================================================
// Company Types
// ============================================================================

export const companySchema = z.object({
  id: z.number(),
  name: z.string(),
  code: z.string(),
  description: z.string().optional(),
  status: z.number(),
  sort_order: z.number().optional(),
  department_count: z.number().optional(),
  user_count: z.number().optional(),
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
})
export type Company = z.infer<typeof companySchema>

export const companyFormSchema = z.object({
  name: z.string().min(1, 'Name is required').max(128),
  code: z.string().min(1, 'Code is required').max(64),
  description: z.string().max(512).default(''),
  status: z.number().default(1),
  sort_order: z.number().default(0),
})
export type CompanyFormValues = z.infer<typeof companyFormSchema>

export const COMPANY_FORM_DEFAULT_VALUES: CompanyFormValues = {
  name: '',
  code: '',
  description: '',
  status: 1,
  sort_order: 0,
}

// ============================================================================
// Department Types
// ============================================================================

export const departmentSchema = z.object({
  id: z.number(),
  company_id: z.number(),
  parent_id: z.number().optional(),
  name: z.string(),
  level: z.number().optional(),
  path: z.string().optional(),
  description: z.string().optional(),
  status: z.number(),
  sort_order: z.number().optional(),
  child_count: z.number().optional(),
  user_count: z.number().optional(),
  company_name: z.string().optional(),
  parent_name: z.string().optional(),
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
})
export type Department = z.infer<typeof departmentSchema>

export const departmentFormSchema = z.object({
  company_id: z.number().min(1, 'Company is required'),
  name: z.string().min(1, 'Name is required').max(128),
  parent_id: z.number().default(0),
  description: z.string().max(512).default(''),
  status: z.number().default(1),
  sort_order: z.number().default(0),
})
export type DepartmentFormValues = z.infer<typeof departmentFormSchema>

export const DEPARTMENT_FORM_DEFAULT_VALUES: DepartmentFormValues = {
  company_id: 0,
  name: '',
  parent_id: 0,
  description: '',
  status: 1,
  sort_order: 0,
}

// ============================================================================
// Common Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PaginatedResponse<T> {
  success: boolean
  message?: string
  data?: {
    items: T[]
    total: number
    page: number
    page_size: number
  }
}

export type CompaniesDialogType = 'create' | 'update' | 'delete'
export type DepartmentsDialogType = 'create' | 'create-child' | 'update' | 'delete'
