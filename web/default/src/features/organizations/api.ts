import { api } from '@/lib/api'
import type {
  Company,
  Department,
  CompanyFormValues,
  DepartmentFormValues,
  ApiResponse,
  PaginatedResponse,
} from './types'

// ============================================================================
// Company APIs
// ============================================================================

export async function getCompanies(params: {
  p?: number
  page_size?: number
  status?: number
}): Promise<PaginatedResponse<Company>> {
  const { p = 1, page_size = 10, status } = params
  let url = `/api/company/?p=${p}&page_size=${page_size}`
  if (status) url += `&status=${status}`
  const res = await api.get(url)
  return res.data
}

export async function getAllCompanies(
  status?: number
): Promise<ApiResponse<Company[]>> {
  let url = '/api/company/all'
  if (status) url += `?status=${status}`
  const res = await api.get(url)
  return res.data
}

export async function getCompany(
  id: number
): Promise<ApiResponse<Company>> {
  const res = await api.get(`/api/company/${id}`)
  return res.data
}

export async function createCompany(
  data: CompanyFormValues
): Promise<ApiResponse<Company>> {
  const res = await api.post('/api/company/', data)
  return res.data
}

export async function updateCompany(
  id: number,
  data: CompanyFormValues
): Promise<ApiResponse<Company>> {
  const res = await api.put(`/api/company/${id}`, data)
  return res.data
}

export async function deleteCompany(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/company/${id}`)
  return res.data
}

export async function updateCompanyStatus(
  id: number,
  status: number
): Promise<ApiResponse> {
  const res = await api.patch(`/api/company/${id}/status`, { status })
  return res.data
}

// ============================================================================
// Department APIs
// ============================================================================

export async function getDepartments(params: {
  p?: number
  page_size?: number
  company_id?: number
  status?: number
}): Promise<PaginatedResponse<Department>> {
  const { p = 1, page_size = 10, company_id, status } = params
  let url = `/api/department/?p=${p}&page_size=${page_size}`
  if (company_id) url += `&company_id=${company_id}`
  if (status) url += `&status=${status}`
  const res = await api.get(url)
  return res.data
}

export async function getAllDepartments(params?: {
  company_id?: number
  status?: number
}): Promise<ApiResponse<Department[]>> {
  let url = '/api/department/all'
  const queryParams: string[] = []
  if (params?.company_id) queryParams.push(`company_id=${params.company_id}`)
  if (params?.status) queryParams.push(`status=${params.status}`)
  if (queryParams.length > 0) url += '?' + queryParams.join('&')
  const res = await api.get(url)
  return res.data
}

export async function getDepartmentTree(
  companyId: number
): Promise<ApiResponse<Department[]>> {
  const res = await api.get(`/api/department/tree?company_id=${companyId}`)
  return res.data
}

export async function getDepartment(
  id: number
): Promise<ApiResponse<Department>> {
  const res = await api.get(`/api/department/${id}`)
  return res.data
}

export async function createDepartment(
  data: DepartmentFormValues
): Promise<ApiResponse<Department>> {
  const res = await api.post('/api/department/', data)
  return res.data
}

export async function updateDepartment(
  id: number,
  data: DepartmentFormValues
): Promise<ApiResponse<Department>> {
  const res = await api.put(`/api/department/${id}`, data)
  return res.data
}

export async function deleteDepartment(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/department/${id}`)
  return res.data
}

export async function moveDepartment(
  id: number,
  targetParentId: number
): Promise<ApiResponse<Department>> {
  const res = await api.post(`/api/department/${id}/move`, {
    target_parent_id: targetParentId,
  })
  return res.data
}

export async function updateDepartmentStatus(
  id: number,
  status: number
): Promise<ApiResponse> {
  const res = await api.patch(`/api/department/${id}/status`, { status })
  return res.data
}

// ============================================================================
// User Department APIs
// ============================================================================

export async function setUserDepartment(
  userId: number,
  companyId?: number,
  departmentId?: number
): Promise<ApiResponse> {
  const data: { company_id?: number; department_id?: number } = {}
  if (companyId !== undefined) data.company_id = companyId
  if (departmentId !== undefined) data.department_id = departmentId
  const res = await api.put(`/api/user/${userId}/department`, data)
  return res.data
}

export async function clearUserDepartment(
  userId: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${userId}/department`)
  return res.data
}
