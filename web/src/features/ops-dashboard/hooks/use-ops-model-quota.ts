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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { getUserQuotaDates } from '@/features/dashboard/api'
import { buildQueryParams, getDefaultDays } from '@/features/dashboard/lib'
import type {
  DashboardFilters,
  QuotaDataItem,
} from '@/features/dashboard/types'
import { requireServerSuccess } from '@/lib/server-error-message'
import { computeTimeRange } from '@/lib/time'

export function useOpsModelQuota(filters?: DashboardFilters) {
  const timeRange = useMemo(
    () =>
      computeTimeRange(
        getDefaultDays(filters?.time_granularity),
        filters?.start_timestamp,
        filters?.end_timestamp
      ),
    [filters]
  )
  const query = useQuery({
    queryKey: [
      'ops',
      'model-quota',
      timeRange.start_timestamp,
      timeRange.end_timestamp,
      filters?.username,
      filters?.exclude_admins === true,
    ],
    queryFn: async () =>
      requireServerSuccess(
        await getUserQuotaDates(
          buildQueryParams(timeRange, {
            time_granularity: filters?.time_granularity,
            username: filters?.username,
            exclude_admins: filters?.exclude_admins,
          }),
          true
        )
      ),
    staleTime: 60 * 1000,
    retry: false,
  })
  return {
    ...query,
    data: (query.data?.data ?? []) as QuotaDataItem[],
    timeRange,
  }
}
