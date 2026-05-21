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
import { api } from '@/lib/api'
import type {
  CreateRateLimitRequest,
  Company,
  CompanyFormData,
  CompanyListResponse,
  Department,
  DepartmentFormData,
  DepartmentListResponse,
  DepartmentTreeNode,
  EffectiveRateLimitData,
  MoveDepartmentFormData,
  OrganizationRateLimit,
  RateLimitListResponse,
  UpdateRateLimitRequest,
  UserDepartmentFormData,
  ApiResponse,
} from './types'

// ============================================================================
// Company Management APIs
// ============================================================================

/**
 * Get paginated companies list
 */
export async function getCompanies(params: {
  page?: number
  page_size?: number
  status?: number
} = {}): Promise<CompanyListResponse> {
  const { page = 1, page_size = 20, status } = params
  const queryParams = new URLSearchParams({
    page: String(page),
    page_size: String(page_size),
  })
  if (status !== undefined) {
    queryParams.set('status', String(status))
  }
  const res = await api.get(`/api/company/?${queryParams.toString()}`)
  return res.data.data
}

/**
 * Get all companies (for dropdown selection)
 */
export async function getAllCompanies(status?: number): Promise<ApiResponse<Company[]>> {
  const queryParams = status !== undefined ? `?status=${status}` : ''
  const res = await api.get(`/api/company/all${queryParams}`)
  return res.data
}

/**
 * Get single company by ID
 */
export async function getCompany(id: number): Promise<ApiResponse<Company>> {
  const res = await api.get(`/api/company/${id}`)
  return res.data
}

/**
 * Create a new company
 */
export async function createCompany(
  data: CompanyFormData
): Promise<ApiResponse<Company>> {
  const res = await api.post('/api/company/', data)
  return res.data
}

/**
 * Update an existing company
 */
export async function updateCompany(
  id: number,
  data: CompanyFormData
): Promise<ApiResponse<Company>> {
  const res = await api.put(`/api/company/${id}`, data)
  return res.data
}

/**
 * Delete a company
 */
export async function deleteCompany(id: number): Promise<ApiResponse<null>> {
  const res = await api.delete(`/api/company/${id}`)
  return res.data
}

/**
 * Update company status
 */
export async function updateCompanyStatus(
  id: number,
  status: number
): Promise<ApiResponse<null>> {
  const res = await api.patch(`/api/company/${id}/status`, { status })
  return res.data
}

/**
 * Get users in a company
 */
export async function getCompanyUsers(
  id: number,
  params: { page?: number; page_size?: number } = {}
): Promise<{ items: any[]; total: number; page: number; page_size: number }> {
  const { page = 1, page_size = 20 } = params
  const res = await api.get(
    `/api/company/${id}/users?page=${page}&page_size=${page_size}`
  )
  return res.data.data
}

// ============================================================================
// Department Management APIs
// ============================================================================

/**
 * Get paginated departments list
 */
export async function getDepartments(params: {
  page?: number
  page_size?: number
  company_id?: number
  status?: number
} = {}): Promise<DepartmentListResponse> {
  const { page = 1, page_size = 20, company_id, status } = params
  const queryParams = new URLSearchParams({
    page: String(page),
    page_size: String(page_size),
  })
  if (company_id) {
    queryParams.set('company_id', String(company_id))
  }
  if (status !== undefined) {
    queryParams.set('status', String(status))
  }
  const res = await api.get(`/api/department/?${queryParams.toString()}`)
  return res.data.data
}

/**
 * Get all departments (for dropdown selection)
 */
export async function getAllDepartments(
  companyId?: number,
  status?: number
): Promise<ApiResponse<Department[]>> {
  const queryParams = new URLSearchParams()
  if (companyId) {
    queryParams.set('company_id', String(companyId))
  }
  if (status !== undefined) {
    queryParams.set('status', String(status))
  }
  const queryString = queryParams.toString()
  const res = await api.get(`/api/department/all${queryString ? `?${queryString}` : ''}`)
  return res.data
}

/**
 * Get department tree
 */
export async function getDepartmentTree(
  companyId: number
): Promise<ApiResponse<DepartmentTreeNode[]>> {
  const res = await api.get(`/api/department/tree?company_id=${companyId}`)
  return res.data
}

/**
 * Get single department by ID
 */
export async function getDepartment(id: number): Promise<ApiResponse<Department>> {
  const res = await api.get(`/api/department/${id}`)
  return res.data
}

/**
 * Create a new department
 */
export async function createDepartment(
  data: DepartmentFormData
): Promise<ApiResponse<Department>> {
  const res = await api.post('/api/department/', data)
  return res.data
}

/**
 * Update an existing department
 */
export async function updateDepartment(
  id: number,
  data: Omit<DepartmentFormData, 'company_id'>
): Promise<ApiResponse<Department>> {
  const res = await api.put(`/api/department/${id}`, data)
  return res.data
}

/**
 * Delete a department
 */
export async function deleteDepartment(id: number): Promise<ApiResponse<null>> {
  const res = await api.delete(`/api/department/${id}`)
  return res.data
}

/**
 * Move department to new parent
 */
export async function moveDepartment(
  id: number,
  data: MoveDepartmentFormData
): Promise<ApiResponse<null>> {
  const res = await api.post(`/api/department/${id}/move`, data)
  return res.data
}

/**
 * Update department status
 */
export async function updateDepartmentStatus(
  id: number,
  status: number
): Promise<ApiResponse<null>> {
  const res = await api.patch(`/api/department/${id}/status`, { status })
  return res.data
}

/**
 * Get users in a department
 */
export async function getDepartmentUsers(
  id: number,
  params: { page?: number; page_size?: number } = {}
): Promise<{ items: any[]; total: number; page: number; page_size: number }> {
  const { page = 1, page_size = 20 } = params
  const res = await api.get(
    `/api/department/${id}/users?page=${page}&page_size=${page_size}`
  )
  return res.data.data
}

// ============================================================================
// User-Department Assignment APIs
// ============================================================================

/**
 * Set user's company and department
 */
export async function setUserDepartment(
  userId: number,
  data: UserDepartmentFormData
): Promise<ApiResponse<null>> {
  const res = await api.put(`/api/user/${userId}/department`, data)
  return res.data
}

/**
 * Clear user's company and department
 */
export async function clearUserDepartment(
  userId: number
): Promise<ApiResponse<null>> {
  const res = await api.delete(`/api/user/${userId}/department`)
  return res.data
}

// ============================================================================
// Organization Rate Limit APIs
// ============================================================================

/**
 * Get organization rate limits
 */
export async function getOrganizationRateLimits(params: {
  org_type: 'company' | 'department'
  org_id: number
  model_id?: number
  status?: number
}): Promise<RateLimitListResponse> {
  const queryParams = new URLSearchParams({
    org_type: params.org_type,
    org_id: String(params.org_id),
  })
  if (params.model_id !== undefined) {
    queryParams.set('model_id', String(params.model_id))
  }
  if (params.status !== undefined) {
    queryParams.set('status', String(params.status))
  }

  const res = await api.get(`/api/rate-limit/?${queryParams.toString()}`)
  return res.data.data
}

/**
 * Get single rate limit rule
 */
export async function getOrganizationRateLimit(
  id: number
): Promise<{ data: OrganizationRateLimit }> {
  const res = await api.get(`/api/rate-limit/${id}`)
  return res.data
}

/**
 * Create rate limit rule
 */
export async function createOrganizationRateLimit(
  data: CreateRateLimitRequest
): Promise<ApiResponse<{ id: number }>> {
  const res = await api.post('/api/rate-limit/', data)
  return res.data
}

/**
 * Update rate limit rule
 */
export async function updateOrganizationRateLimit(
  id: number,
  data: UpdateRateLimitRequest
): Promise<ApiResponse<null>> {
  const res = await api.put(`/api/rate-limit/${id}`, data)
  return res.data
}

/**
 * Delete rate limit rule
 */
export async function deleteOrganizationRateLimit(
  id: number
): Promise<ApiResponse<null>> {
  const res = await api.delete(`/api/rate-limit/${id}`)
  return res.data
}

/**
 * Get user's effective rate limit
 */
export async function getUserEffectiveRateLimit(
  userId: number,
  params: { modelId?: number; modelName?: string } = {}
): Promise<ApiResponse<EffectiveRateLimitData>> {
  const queryParams = new URLSearchParams()
  if (params.modelId !== undefined) {
    queryParams.set('model_id', String(params.modelId))
  }
  if (params.modelName) {
    queryParams.set('model_name', params.modelName)
  }
  const queryString = queryParams.toString()
  const url = queryString
    ? `/api/rate-limit/user/${userId}?${queryString}`
    : `/api/rate-limit/user/${userId}`
  const res = await api.get(url)
  return res.data
}
