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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { handleServerError } from '@/lib/handle-server-error'

import {
  createOperatingExpense,
  deleteOperatingExpense,
  getOperatingExpenses,
  getOperatingFlow,
  getOperatingSummary,
} from '../api'
import type { OperatingExpensePayload, OperatingRange } from '../types'

export const operatingSummaryQueryKey = (range: OperatingRange) =>
  [
    'operating',
    'summary',
    range.startTimestamp,
    range.endTimestamp,
  ] as const

export function useOperatingSummary(range: OperatingRange) {
  return useQuery({
    queryKey: operatingSummaryQueryKey(range),
    queryFn: () => getOperatingSummary(range),
    staleTime: 60 * 1000,
  })
}

export function useOperatingExpenses(range: OperatingRange) {
  return useQuery({
    queryKey: ['operating', 'expenses', range.startTimestamp, range.endTimestamp],
    queryFn: () => getOperatingExpenses(range),
    staleTime: 60 * 1000,
  })
}

export function useCreateOperatingExpense() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: OperatingExpensePayload) =>
      createOperatingExpense(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['operating'] })
    },
    onError: (error) => handleServerError(error),
  })
}

export function useDeleteOperatingExpense() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => deleteOperatingExpense(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['operating'] })
    },
    onError: (error) => handleServerError(error),
  })
}

export function useOperatingFlow(
  range: OperatingRange,
  page: number,
  pageSize: number
) {
  return useQuery({
    queryKey: [
      'operating',
      'flow',
      range.startTimestamp,
      range.endTimestamp,
      page,
      pageSize,
    ],
    queryFn: () => getOperatingFlow(range, page, pageSize),
    staleTime: 60 * 1000,
  })
}
