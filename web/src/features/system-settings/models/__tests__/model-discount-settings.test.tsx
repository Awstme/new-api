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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ModelDiscountSettingsCard } from '../model-discount-settings-card'

function renderCard(defaultValue: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ModelDiscountSettingsCard defaultValue={defaultValue} />
    </QueryClientProvider>
  )
}

describe('model discount settings card', () => {
  it('renders configured groups with their discount and models', () => {
    renderCard(
      '[{"id":"g1","name":"VIP","discount":0.3,"models":["m1"],"enabled":true}]'
    )

    expect(screen.getByDisplayValue('VIP')).toBeVisible()
    expect(screen.getByText('30%')).toBeVisible()
    expect(screen.getByText('m1')).toBeVisible()
  })

  it('adds and removes discount groups', () => {
    renderCard('[]')

    expect(screen.getByText('No discount groups configured')).toBeVisible()
    fireEvent.click(screen.getByRole('button', { name: 'Add discount group' }))
    expect(screen.getAllByPlaceholderText('Group name')).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(screen.queryByPlaceholderText('Group name')).toBeNull()
  })
})
