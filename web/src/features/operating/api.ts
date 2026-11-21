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
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  OperatingExpense,
  OperatingExpensePayload,
  OperatingFlowPage,
  OperatingRange,
  OperatingRefund,
  OperatingRefundPayload,
  OperatingSummary,
} from './types'

function rangeParams(range: OperatingRange) {
  return {
    start_timestamp: range.startTimestamp,
    end_timestamp: range.endTimestamp,
  }
}

export async function getOperatingSummary(range: OperatingRange) {
  const res = await api.get<{ success: boolean; data: OperatingSummary }>(
    '/api/operating/summary',
    { params: rangeParams(range) }
  )
  return requireServerSuccess(res.data).data
}

export async function getOperatingExpenses(range: OperatingRange) {
  const res = await api.get<{ success: boolean; data: OperatingExpense[] }>(
    '/api/operating/expenses',
    { params: rangeParams(range) }
  )
  return requireServerSuccess(res.data).data
}

export async function createOperatingExpense(payload: OperatingExpensePayload) {
  const res = await api.post<{ success: boolean; data: OperatingExpense }>(
    '/api/operating/expenses',
    payload
  )
  return requireServerSuccess(res.data).data
}

export async function deleteOperatingExpense(id: number) {
  requireServerSuccess(
    (await api.delete<{ success: boolean }>(`/api/operating/expenses/${id}`)).data
  )
}

export async function getOperatingFlow(
  range: OperatingRange,
  page: number,
  pageSize: number
) {
  const res = await api.get<{ success: boolean; data: OperatingFlowPage }>(
    '/api/operating/flow',
    { params: { ...rangeParams(range), page, page_size: pageSize } }
  )
  return requireServerSuccess(res.data).data
}

export async function getOperatingRefunds(range: OperatingRange) {
  const res = await api.get<{ success: boolean; data: OperatingRefund[] }>(
    '/api/operating/refunds',
    { params: rangeParams(range) }
  )
  return requireServerSuccess(res.data).data
}

export async function createOperatingRefund(payload: OperatingRefundPayload) {
  const res = await api.post<{ success: boolean; data: OperatingRefund }>(
    '/api/operating/refunds',
    payload
  )
  return requireServerSuccess(res.data).data
}

export async function deleteOperatingRefund(id: number) {
  requireServerSuccess(
    (await api.delete<{ success: boolean }>(`/api/operating/refunds/${id}`)).data
  )
}
