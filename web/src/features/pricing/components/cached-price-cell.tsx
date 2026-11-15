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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import { useBillingTime } from '../hooks/use-billing-time'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
  isUnconfiguredTaskUsageModel,
} from '../lib/dynamic-price'
import { getModelDiscountRatio, isTokenBasedModel } from '../lib/model-helpers'
import { withBasePricing } from '../lib/plugin-pricing'
import { formatPrice, stripTrailingZeros } from '../lib/price'
import type { PricingModel } from '../types'
import type { ModelPriceCellOptions } from './model-price-cell'

export function CachedPriceCell(props: {
  model: PricingModel
  options: ModelPriceCellOptions
}) {
  const { t } = useTranslation()
  const {
    tokenUnit = DEFAULT_TOKEN_UNIT,
    priceRate = 1,
    usdExchangeRate = 1,
    showRechargePrice = false,
    selectedGroup,
  } = props.options

  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'

  const model = useMemo(() => withBasePricing(props.model), [props.model])
  const currency = useSystemConfigStore((state) => state.config.currency)
  const billingTime = useBillingTime(model.billing_expr)
  const baseGroupRatio = getDynamicDisplayGroupRatio(model, selectedGroup)
  const discountRatio = getModelDiscountRatio(model)
  const hasDiscount = discountRatio < 1
  const dynamicSummary = useMemo(
    () =>
      getDynamicPricingSummary(model, {
        now: billingTime === undefined ? undefined : new Date(billingTime),
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: baseGroupRatio,
      }),
    // Currency is read indirectly by the price formatter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      model,
      tokenUnit,
      showRechargePrice,
      priceRate,
      usdExchangeRate,
      baseGroupRatio,
      billingTime,
      currency,
    ]
  )
  const discountedSummary = useMemo(
    () =>
      hasDiscount
        ? getDynamicPricingSummary(model, {
            now: billingTime === undefined ? undefined : new Date(billingTime),
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            groupRatioMultiplier: baseGroupRatio * discountRatio,
          })
        : undefined,
    // Currency is read indirectly by the price formatter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      model,
      tokenUnit,
      showRechargePrice,
      priceRate,
      usdExchangeRate,
      baseGroupRatio,
      discountRatio,
      hasDiscount,
      billingTime,
      currency,
    ]
  )
  const displaySummary = hasDiscount ? discountedSummary : dynamicSummary

  if (displaySummary) {
    if (displaySummary.isSpecialExpression) {
      return (
        <span className='text-muted-foreground/50 text-xs'>
          {t('Special billing expression')}
        </span>
      )
    }

    const cacheEntries = displaySummary.entries.filter(
      (entry) =>
        entry.field === 'cacheReadPrice' || entry.field === 'imageCachePrice'
    )
    if (!cacheEntries.length) {
      return <span className='text-muted-foreground/30 text-xs'>—</span>
    }

    return (
      <div className='max-w-full min-w-0'>
        {cacheEntries.map((entry) => {
          const originalEntry = hasDiscount
            ? dynamicSummary?.entries.find(
                (candidate) => candidate.field === entry.field
              )
            : undefined
          return (
            <div
              key={entry.field}
              className='flex flex-wrap items-baseline gap-x-1'
            >
              {(cacheEntries.length > 1 ||
                entry.field === 'imageCachePrice') && (
                <span className='text-muted-foreground text-xs'>
                  {entry.field === 'imageCachePrice'
                    ? t('Image Cache')
                    : t('Cache Read')}
                </span>
              )}
              <span className='font-mono text-sm tabular-nums'>
                {stripTrailingZeros(entry.formatted)}
              </span>
              {originalEntry && (
                <span
                  className='text-muted-foreground/60 font-mono text-xs tabular-nums line-through'
                  title={t('Original price')}
                >
                  {stripTrailingZeros(originalEntry.formatted)}
                </span>
              )}
            </div>
          )
        })}
        <div className='text-muted-foreground/50 text-[10px]'>
          / {tokenUnitLabel}
        </div>
        {hasDiscount && model.discount_group && (
          <Badge variant='warning' className='mt-1'>
            {model.discount_group}
          </Badge>
        )}
      </div>
    )
  }

  if (isUnconfiguredTaskUsageModel(model)) {
    return <span className='text-muted-foreground/30 text-xs'>—</span>
  }

  const isTokenBased = isTokenBasedModel(model)

  if (!isTokenBased || model.cache_ratio == null) {
    return <span className='text-muted-foreground/30 text-xs'>—</span>
  }

  const cachedPrice = stripTrailingZeros(
    formatPrice(
      model,
      'cache',
      tokenUnit,
      showRechargePrice,
      priceRate,
      usdExchangeRate,
      selectedGroup,
      true,
      hasDiscount ? discountRatio : 1
    )
  )
  const originalCachedPrice = hasDiscount
    ? stripTrailingZeros(
        formatPrice(
          model,
          'cache',
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate,
          selectedGroup
        )
      )
    : undefined

  return (
    <div className='max-w-full min-w-0'>
      <span className='font-mono text-sm tabular-nums'>{cachedPrice}</span>
      {originalCachedPrice && (
        <span
          className='text-muted-foreground/60 ml-1 font-mono text-xs tabular-nums line-through'
          title={t('Original price')}
        >
          {originalCachedPrice}
        </span>
      )}
      <div className='text-muted-foreground/50 text-[10px]'>
        / {tokenUnitLabel}
      </div>
      {hasDiscount && model.discount_group && (
        <Badge variant='warning' className='mt-1'>
          {model.discount_group}
        </Badge>
      )}
    </div>
  )
}
