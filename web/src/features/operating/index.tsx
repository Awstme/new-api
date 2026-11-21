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
import { useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  OperatingExpenseTable,
  OperatingGrantTable,
  OperatingRefundTable,
  OperatingSummaryCards,
} from './components/operating-cards'
import { OperatingFlowTable } from './components/operating-flow-table'
import { OperatingHeader } from './components/operating-header'
import {
  beijingDayKey,
  buildOperatingRange,
  OPERATING_DEFAULT_RANGE_DAYS,
} from './constants'
import { useOperatingSummary } from './hooks/use-operating'
import {
  type OperatingSectionId,
  OPERATING_DEFAULT_SECTION,
  OPERATING_SECTION_IDS,
} from './section-registry'

const SECTION_META = {
  overview: { titleKey: 'Operating Analysis' },
  flow: { titleKey: 'Flow records' },
} as const

function OperatingOverview(props: { activePreset: number; onPresetChange: (days: number) => void }) {
  const { t } = useTranslation()
  const range = useMemo(() => buildOperatingRange(props.activePreset), [props.activePreset])
  const summaryQuery = useOperatingSummary(range)

  if (summaryQuery.isError) {
    return <ErrorState title={t('Failed to load operating data')} />
  }

  return (
    <div className='space-y-3 sm:space-y-4'>
      <OperatingHeader
        range={range}
        activePreset={props.activePreset}
        onPresetChange={props.onPresetChange}
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
      <OperatingRefundTable
        summary={summaryQuery.data}
        range={range}
        loading={summaryQuery.isLoading}
      />
    </div>
  )
}

export function Operating() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = useParams({ from: '/_authenticated/operating/$section' })
  const [activePreset, setActivePreset] = useState(OPERATING_DEFAULT_RANGE_DAYS)

  const requestedSection = params.section as string
  const section: OperatingSectionId = (
    OPERATING_SECTION_IDS as readonly string[]
  ).includes(requestedSection)
    ? (requestedSection as OperatingSectionId)
    : OPERATING_DEFAULT_SECTION
  // 「今日」是相对现在的区间；跨日或跨月后必须重算，否则页面会一直停在旧区间。
  const [rangeAnchor, setRangeAnchor] = useState(() => beijingDayKey())
  const range = useMemo(
    () => buildOperatingRange(activePreset, new Date(rangeAnchor)),
    [activePreset, rangeAnchor]
  )

  useEffect(() => {
    const timer = setInterval(() => {
      const key = beijingDayKey()
      setRangeAnchor((prev) => (prev === key ? prev : key))
    }, 60 * 1000)
    return () => clearInterval(timer)
  }, [])

  const handleSectionChange = (next: string) => {
    void navigate({
      to: '/operating/$section',
      params: { section: next },
    })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t(SECTION_META[section].titleKey)}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-3 sm:space-y-4'>
          <Tabs value={section} onValueChange={handleSectionChange}>
            <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
              {OPERATING_SECTION_IDS.map((id) => (
                <TabsTrigger key={id} value={id}>
                  {t(SECTION_META[id].titleKey)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
          {section === 'overview' ? (
            <OperatingOverview
              activePreset={activePreset}
              onPresetChange={setActivePreset}
            />
          ) : (
            <div className='space-y-3 sm:space-y-4'>
              <OperatingHeader
                range={range}
                activePreset={activePreset}
                onPresetChange={setActivePreset}
                showExpenseAction={false}
              />
              <OperatingFlowTable range={range} />
            </div>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
