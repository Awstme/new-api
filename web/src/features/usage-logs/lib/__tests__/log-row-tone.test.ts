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
import { describe, expect, it } from 'vitest'

import { LOG_TYPE_ENUM } from '../../constants'
import { getCommonLogRowTone } from '../log-row-tone'

function other(value: Record<string, unknown>) {
  return JSON.stringify(value)
}

describe('getCommonLogRowTone', () => {
  it('marks error rows as errors', () => {
    expect(getCommonLogRowTone({ type: LOG_TYPE_ENUM.ERROR }, false)).toBe(
      'error'
    )
  })

  it('marks successful session rows with errors as errors', () => {
    expect(
      getCommonLogRowTone(
        {
          type: LOG_TYPE_ENUM.CONSUME,
          other: other({ session_error_count: 2 }),
        },
        false
      )
    ).toBe('error')
  })

  it('keeps a session without errors untinted', () => {
    expect(
      getCommonLogRowTone(
        {
          type: LOG_TYPE_ENUM.CONSUME,
          other: other({ session_error_count: 0 }),
        },
        false
      )
    ).toBeNull()
  })

  it('keeps refund and saturation precedence', () => {
    expect(getCommonLogRowTone({ type: LOG_TYPE_ENUM.REFUND }, false)).toBe(
      'refund'
    )
    expect(
      getCommonLogRowTone(
        {
          type: LOG_TYPE_ENUM.CONSUME,
          other: other({
            admin_info: { quota_saturation: { op: 'mul', kind: 'overflow' } },
          }),
        },
        true
      )
    ).toBe('saturation')
  })
})
