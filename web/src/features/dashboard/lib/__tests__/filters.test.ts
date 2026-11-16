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
import { beforeEach, describe, expect, it } from 'vitest'

import {
  buildQueryParams,
  getSavedExcludeAdmins,
  saveExcludeAdmins,
} from '../filters'

describe('dashboard query filters', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('passes the exclude-admins flag only when enabled', () => {
    const range = { start_timestamp: 1, end_timestamp: 2 }
    expect(buildQueryParams(range, { exclude_admins: true })).toMatchObject({
      exclude_admins: true,
    })
    expect(
      buildQueryParams(range, { exclude_admins: false })
    ).not.toHaveProperty('exclude_admins')
  })

  it('persists the exclude-admins preference', () => {
    expect(getSavedExcludeAdmins()).toBe(false)
    saveExcludeAdmins(true)
    expect(getSavedExcludeAdmins()).toBe(true)
    saveExcludeAdmins(false)
    expect(getSavedExcludeAdmins()).toBe(false)
  })
})
