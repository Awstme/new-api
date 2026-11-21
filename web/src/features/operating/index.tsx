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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'

import {
  OperatingExpenseTable,
  OperatingGrantTable,
  OperatingSummaryCards,
} from './components/operating-cards'
import { OperatingHeader } from './components/operating-header'
import { buildOperatingRange } from './constants'
import { useOperatingSummary } from './hooks/use-operating'

export function Operating() {
  const { t } = useTranslation()
  const [activePreset, setActivePreset] = useState(0)

  const range = useMemo(() => buildOperatingRange(activePreset), [activePreset])
  const summaryQuery = useOperatingSummary(range)

  const content = summaryQuery.isError ? (
    <ErrorState title={t('Failed to load operating data')} />
  ) : (
    <div className='space-y-3 sm:space-y-4'>
      <OperatingHeader
        range={range}
        activePreset={activePreset}
        onPresetChange={setActivePreset}
      />
      {summaryQuery.data?.logs_available === false ? (
        <div className='text-muted-foreground rounded-lg border px-3 py-2 text-xs sm:px-5'>
          {t(
            'Consume and grant logs are unavailable for this deployment, so those figures are incomplete rather than zero.'
          )}
        </div>
      ) : null}
      <OperatingSummaryCards
        summary={summaryQuery.data}
        range={range}
        loading={summaryQuery.isLoading}
      />
      <div className='grid gap-3 sm:gap-4 xl:grid-cols-2'>
        <OperatingGrantTable
          summary={summaryQuery.data}
          range={range}
          loading={summaryQuery.isLoading}
        />
        <OperatingExpenseTable
          summary={summaryQuery.data}
          range={range}
          loading={summaryQuery.isLoading}
        />
      </div>
    </div>
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Operating Analysis')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
