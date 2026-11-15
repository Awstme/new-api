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
import { describe, expect, test } from 'vitest'

import { buildSearchParams } from '../filter'
import { buildApiParams } from '../utils'

describe('usage log filter params', () => {
  test('forwards the fuzzy query to the API for admins and users', () => {
    const adminParams = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { q: 'alice 403', model: 'gpt-4o' },
      isAdmin: true,
    })
    expect(adminParams).toMatchObject({ q: 'alice 403', model_name: 'gpt-4o' })

    const userParams = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { q: 'alice 403' },
      isAdmin: false,
    })
    expect(userParams.q).toBe('alice 403')
  })

  test('keeps the query in the URL search state', () => {
    const startTime = new Date(2026, 10, 14, 8, 0, 0)
    const endTime = new Date(2026, 10, 14, 9, 0, 0)
    const params = buildSearchParams(
      { startTime, endTime, q: 'needle', model: 'gpt-4o' },
      'common'
    )
    expect(params.q).toBe('needle')
    expect(params.model).toBe('gpt-4o')

    const cleared = buildSearchParams(
      { startTime, endTime, q: '', model: 'gpt-4o' },
      'common'
    )
    expect(cleared.q).toBeUndefined()
  })
})
