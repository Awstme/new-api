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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { LogStatCards } from '../log-stat-cards'

afterEach(cleanup)

it('keeps user consumption when a cached site-wide total exists', async () => {
  useAuthStore
    .getState()
    .auth.setUser({
      id: 1,
      username: 'admin',
      role: 10,
      quota: 0,
      used_quota: 0,
      request_count: 0,
    })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['dashboard', 'free-quota', 100, 200], {
    success: true,
    data: { consumption_total: 9000000, free_total: 4000000 },
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [{ created_at: 150, quota: 500000, count: 1 }],
    },
  })
  const view = render(
    <QueryClientProvider client={client}>
      <LogStatCards
        filters={{
          username: 'alice',
          start_timestamp: new Date(100000),
          end_timestamp: new Date(200000),
        }}
      />
    </QueryClientProvider>
  )
  expect(await screen.findByTitle(formatQuota(500000))).toBeInTheDocument()
  expect(screen.queryByTitle(formatQuota(9000000))).not.toBeInTheDocument()
  view.unmount()
  client.clear()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
})
