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
export interface DashboardLogHealthBucket {
  t: number
  requests: number
  success: number
  failed: number
  retried: number
  quota: number
  tokens: number
  failed_4xx: number
  failed_5xx: number
  interrupted: number
  p95: number
}

export interface DashboardLogHealthSummary {
  quota: number
  requests: number
  success_requests: number
  failed_requests: number
  failed_4xx: number
  failed_5xx: number
  interrupted_requests: number
  status_codes: Record<string, number>
  retried_requests: number
  error_rows: number
  error_rate: number
  tokens: number
  p50: number
  p95: number
  p99: number
  avg_use_time: number
  series: DashboardLogHealthBucket[]
}

export interface FreeQuotaUser {
  user_id: number
  username: string
  granted: number
  used: number
  grant_count: number
  last_used_at: number
  method: 'exact' | 'log_text' | 'config' | 'mixed' | string
}

export interface FreeQuotaSource {
  key: string
  kind: 'free' | 'paid' | string
  quota: number
  granted: number
  count: number
  grant_count: number
  method: 'exact' | 'log_text' | 'config' | 'mixed' | string
  users: FreeQuotaUser[]
}

export interface OnlineTopupRecord {
  user_id: number
  username: string
  created_at: number
  quota: number
  amount: number
  payment_method: string
  request_id: string
}

export interface FreeQuotaStats {
  total: number
  free_total: number
  consumption_total: number
  online_topup_count: number
  online_topup_quota: number
  online_topup_amount: number
  online_topup_records: OnlineTopupRecord[]
  granted: number
  sources: FreeQuotaSource[]
}
