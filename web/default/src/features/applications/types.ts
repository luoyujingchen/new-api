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
// ============================================================================
// Application Management Types
// ============================================================================

export interface Application {
  id: number
  app_key: string
  name: string
  description: string
  status: number
  sort_order: number
  header_validation_rules?: ApplicationHeaderValidationRule[]
  header_match_required: boolean
  created_at: number
  updated_at: number
  token_count?: number
}

export type ApplicationHeaderOperator = 'equals' | 'one_of'

export interface ApplicationHeaderValidationRule {
  header: string
  operator: ApplicationHeaderOperator
  value?: string
  values?: string[]
}

export interface ApplicationFormData {
  name: string
  description?: string
  status: number
  sort_order?: number
  header_validation_rules?: ApplicationHeaderValidationRule[]
  header_match_required?: boolean
}

export interface ApplicationListResponse {
  items: Application[]
  total: number
  page: number
  page_size: number
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetApplicationsParams {
  page?: number
  page_size?: number
  status?: number
}
