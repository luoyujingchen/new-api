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
  Application,
  ApplicationFormData,
  ApplicationListResponse,
  ApiResponse,
  GetApplicationsParams,
} from './types'

// ============================================================================
// Application Management APIs
// ============================================================================

/**
 * Get paginated applications list
 */
export async function getApplications(
  params: GetApplicationsParams = {}
): Promise<ApplicationListResponse> {
  const { page = 1, page_size = 20, status } = params
  const queryParams = new URLSearchParams({
    page: String(page),
    page_size: String(page_size),
  })
  if (status !== undefined) {
    queryParams.set('status', String(status))
  }
  const res = await api.get(`/api/application/?${queryParams.toString()}`)
  return res.data.data
}

/**
 * Get all applications (for dropdown selection)
 */
export async function getAllApplications(
  status?: number
): Promise<ApiResponse<Application[]>> {
  const queryParams = status !== undefined ? `?status=${status}` : ''
  const res = await api.get(`/api/application/all${queryParams}`)
  return res.data
}

/**
 * Get selectable applications for authenticated users.
 */
export async function getSelectableApplications(): Promise<
  ApiResponse<Application[]>
> {
  const res = await api.get('/api/application/self/all')
  return res.data
}

/**
 * Get single application by ID
 */
export async function getApplication(
  id: number
): Promise<ApiResponse<Application>> {
  const res = await api.get(`/api/application/${id}`)
  return res.data
}

/**
 * Create a new application
 */
export async function createApplication(
  data: ApplicationFormData
): Promise<ApiResponse<Application>> {
  const res = await api.post('/api/application/', data)
  return res.data
}

/**
 * Update an existing application
 */
export async function updateApplication(
  id: number,
  data: ApplicationFormData
): Promise<ApiResponse<Application>> {
  const res = await api.put(`/api/application/${id}`, data)
  return res.data
}

/**
 * Delete an application
 */
export async function deleteApplication(
  id: number
): Promise<ApiResponse<null>> {
  const res = await api.delete(`/api/application/${id}`)
  return res.data
}

/**
 * Update application status
 */
export async function updateApplicationStatus(
  id: number,
  status: number
): Promise<ApiResponse<null>> {
  const res = await api.patch(`/api/application/${id}/status`, { status })
  return res.data
}
