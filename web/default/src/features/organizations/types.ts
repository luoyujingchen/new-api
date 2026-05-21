/**
 * Organization Management Types
 */

// Company types
export interface Company {
  id: number;
  name: string;
  code: string;
  description?: string;
  status: number; // 1 = enabled, 0 = disabled
  sort_order: number;
  created_at: number;
  updated_at: number;
  department_count?: number;
  user_count?: number;
}

export interface CompanyFormData {
  name: string;
  code: string;
  description?: string;
  status: number;
  sort_order: number;
}

// Department types
export interface Department {
  id: number;
  company_id: number;
  name: string;
  parent_id: number | null;
  level: number; // 1-4
  path: string; // e.g., "/1/5/12"
  description?: string;
  status: number; // 1 = enabled, 0 = disabled
  sort_order: number;
  created_at: number;
  updated_at: number;
  company?: CompanySummary;
  parent?: DepartmentSummary;
  child_count?: number;
  user_count?: number;
}

export interface DepartmentTreeNode extends Department {
  children?: DepartmentTreeNode[];
}

export interface DepartmentFormData {
  company_id: number;
  name: string;
  parent_id?: number | null;
  description?: string;
  status: number;
  sort_order: number;
}

export interface MoveDepartmentFormData {
  parent_id?: number | null;
}

// Summary types for nested display
export interface CompanySummary {
  id: number;
  name: string;
  code: string;
}

export interface DepartmentSummary {
  id: number;
  name: string;
  path: string;
}

// User-Department types
export interface UserDepartmentFormData {
  company_id?: number | null;
  department_id?: number | null;
}

// List response types
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page?: number;
  page_size?: number;
}

export interface ApiResponse<T = unknown> {
  success: boolean;
  message?: string;
  data?: T;
}

export interface CompanyListResponse extends PaginatedResponse<Company> {}
export interface DepartmentListResponse extends PaginatedResponse<Department> {}

// Status options
export const STATUS_OPTIONS = [
  { value: 1, label: 'Enabled', labelZh: '启用' },
  { value: 0, label: 'Disabled', labelZh: '禁用' },
] as const;

// Department level labels
export const DEPARTMENT_LEVEL_LABELS: Record<number, { en: string; zh: string }> = {
  1: { en: 'Level 1', zh: '一级部门' },
  2: { en: 'Level 2', zh: '二级部门' },
  3: { en: 'Level 3', zh: '三级部门' },
  4: { en: 'Level 4', zh: '四级部门' },
};

// Rate Limit types
export interface TimeSlot {
  start_time: string; // HH:MM format
  end_time: string;   // HH:MM format
  weekdays?: number[]; // 0-6, 0 = Sunday
}

export interface OrganizationRateLimit {
  id: number;
  org_type: 'company' | 'department';
  org_id: number;
  org_name?: string;
  model_id?: number;
  model_name?: string;
  time_slots: TimeSlot[];
  rpms: number[];
  priority: number;
  status: number;
  created_at: number;
  updated_at: number;
}

export interface CreateRateLimitRequest {
  org_type: 'company' | 'department';
  org_id: number;
  model_id?: number;
  model_name?: string;
  time_slots: TimeSlot[];
  rpms: number[];
  priority?: number;
  status?: number;
}

export interface UpdateRateLimitRequest {
  time_slots: TimeSlot[];
  rpms: number[];
  priority?: number;
  status?: number;
}

export interface EffectiveRateLimitData {
  source: 'department' | 'company' | 'group' | 'global' | 'none';
  org_name?: string;
  org_type?: string;
  org_id?: number;
  model_id?: number;
  model_name?: string;
  time_slot?: TimeSlot;
  rpm: number;
  weekday?: number;
}

export interface RateLimitListResponse {
  items: OrganizationRateLimit[];
  total: number;
}
