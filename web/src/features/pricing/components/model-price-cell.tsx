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
import { getCurrencyLabel } from '@/lib/currency'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import { useBillingTime } from '../hooks/use-billing-time'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPriceUnitLabelKey,
  getDynamicPricingSummary,
  isUnconfiguredTaskUsageModel,
} from '../lib/dynamic-price'
import { getModelDiscountRatio, isTokenBasedModel } from '../lib/model-helpers'
import { withBasePricing } from '../lib/plugin-pricing'
import { formatPrice, formatRequestPrice } from '../lib/price'
import { taskUsageUnitLabel } from '../lib/task-price-display'
import type { PricingModel, TokenUnit } from '../types'

export type ModelPriceCellOptions = {
  tokenUnit?: TokenUnit
  priceRate?: number
  usdExchangeRate?: number
  showRechargePrice?: boolean
  selectedGroup?: string
}

type PriceMetric = {
  label: string
  value: string
  originalValue?: string
}

export function ModelPriceCell(props: {
  model: PricingModel
  options?: ModelPriceCellOptions
  showExpression?: boolean
}) {
  const { t, i18n } = useTranslation()
  const model = useMemo(() => withBasePricing(props.model), [props.model])
  const currency = useSystemConfigStore((state) => state.config.currency)
  const currencyLabel =
    currency.quotaDisplayType === 'TOKENS' ? 'USD' : getCurrencyLabel()
  const options = props.options ?? {}
  const tokenUnit = options.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const billingTime = useBillingTime(model.billing_expr)
  const discountRatio = getModelDiscountRatio(model)
  const hasDiscount = discountRatio < 1
  const groupRatioMultiplier =
    getDynamicDisplayGroupRatio(model, options.selectedGroup) * discountRatio
  const dynamic = useMemo(
    () =>
      getDynamicPricingSummary(model, {
        priceRate: options.priceRate,
        usdExchangeRate: options.usdExchangeRate,
        showRechargePrice: options.showRechargePrice,
        now: billingTime === undefined ? undefined : new Date(billingTime),
        tokenUnit,
        showCurrencySymbol: false,
        groupRatioMultiplier,
      }),
    // Currency is read indirectly by the price formatter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      model,
      tokenUnit,
      options.priceRate,
      options.usdExchangeRate,
      options.showRechargePrice,
      options.selectedGroup,
      billingTime,
      currency,
    ]
  )
  const discountedDynamic = useMemo(
    () =>
      hasDiscount
        ? getDynamicPricingSummary(model, {
            priceRate: options.priceRate,
            usdExchangeRate: options.usdExchangeRate,
            showRechargePrice: options.showRechargePrice,
            now: billingTime === undefined ? undefined : new Date(billingTime),
            tokenUnit,
            showCurrencySymbol: false,
            groupRatioMultiplier: getDynamicDisplayGroupRatio(
              model,
              options.selectedGroup
            ),
          })
        : undefined,
    // Currency is read indirectly by the price formatter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      model,
      tokenUnit,
      options.priceRate,
      options.usdExchangeRate,
      options.showRechargePrice,
      options.selectedGroup,
      discountRatio,
      hasDiscount,
      billingTime,
      currency,
    ]
  )
  const displayDynamic = hasDiscount ? discountedDynamic : dynamic

  const buildDynamicMetrics = (
    summary: NonNullable<ReturnType<typeof getDynamicPricingSummary>>
  ): PriceMetric[] => {
    const hasRequestPrice = summary.primaryEntries.some(
      (entry) => entry.unit === 'request' || entry.unit === 'image'
    )
    return summary.primaryEntries
      .slice(0, hasRequestPrice ? 3 : 2)
      .map((entry) => {
        const unit = getDynamicPriceUnitLabelKey(entry)
        const unitLabel = taskUsageUnitLabel(
          entry,
          i18n.language,
          unit ? t(unit) : ''
        )
        let suffix = unitLabel ? `/${unitLabel}` : ''
        if (hasRequestPrice && entry.unit === 'token') {
          suffix = `/${t('{{unit}} tokens', { unit: tokenUnitLabel })}`
        }
        return {
          label:
            entry.labelKind === 'schema'
              ? entry.shortLabel
              : t(entry.shortLabel),
          value: `${entry.formattedRange ?? entry.formatted}${suffix}`,
        }
      })
  }

  let metrics: PriceMetric[]
  const providerCaption = displayDynamic?.providerCount
    ? t('{{count}} providers', { count: displayDynamic.providerCount })
    : ''
  const unconfiguredCaption = displayDynamic?.hasUnconfiguredProviders
    ? t('Not configured for some providers')
    : ''
  let caption = t('{{currency}} / {{unit}} tokens', {
    currency: currencyLabel,
    unit: tokenUnitLabel,
  })

  if (displayDynamic) {
    if (displayDynamic.isSpecialExpression) {
      return (
        <span className='block max-w-full min-w-0'>
          <span className='text-muted-foreground block truncate text-sm'>
            {t('Special billing expression')}
          </span>
          {providerCaption && (
            <span className='text-muted-foreground block text-xs'>
              {[providerCaption, unconfiguredCaption]
                .filter(Boolean)
                .join(' · ')}
            </span>
          )}
          {props.showExpression !== false && (
            <code className='text-muted-foreground mt-1 line-clamp-2 block text-xs break-all whitespace-normal'>
              {displayDynamic.rawExpression}
            </code>
          )}
        </span>
      )
    }
    const hasRequestPrice = displayDynamic.primaryEntries.some(
      (entry) => entry.unit === 'request' || entry.unit === 'image'
    )
    metrics = buildDynamicMetrics(displayDynamic)
    if (hasDiscount && dynamic) {
      const originalMetrics = buildDynamicMetrics(dynamic)
      metrics = metrics.map((metric, index) => ({
        ...metric,
        originalValue: originalMetrics[index]?.value,
      }))
    }
    if (metrics.length === 0) {
      return (
        <span className='text-muted-foreground text-sm'>
          {displayDynamic.hasUnconfiguredProviders
            ? t('Not configured')
            : t('Dynamic Pricing')}
          {providerCaption && (
            <span className='block text-xs'>
              {[providerCaption, unconfiguredCaption]
                .filter(Boolean)
                .join(' · ')}
            </span>
          )}
        </span>
      )
    }
    if (displayDynamic.isTaskUsage || hasRequestPrice) caption = currencyLabel
    if (displayDynamic.isTimePricing) {
      caption += ` · ${t('Current period price')}`
    }
    if (displayDynamic.isMixedBilling) {
      caption += ` · ${t('Token or per-call pricing')}`
    }
    if (providerCaption) caption += ` · ${providerCaption}`
    if (unconfiguredCaption) caption += ` · ${unconfiguredCaption}`
    if (displayDynamic.tierCount > 1) {
      caption += ` · ${t('{{count}} tiers', {
        count: displayDynamic.tierCount,
      })}`
    }
  } else {
    if (isUnconfiguredTaskUsageModel(model)) {
      return (
        <span className='text-muted-foreground text-sm'>
          {t('Not configured')}
        </span>
      )
    }
    const tokenBased = isTokenBasedModel(model)
    if (!Number.isFinite(tokenBased ? model.model_ratio : model.model_price)) {
      return (
        <span className='text-muted-foreground text-sm'>
          {t('Unset price')}
        </span>
      )
    }
    const extraMultiplier = hasDiscount ? discountRatio : 1
    if (tokenBased) {
      metrics = [
        {
          label: t('Input'),
          value: formatPrice(
            model,
            'input',
            tokenUnit,
            options.showRechargePrice,
            options.priceRate,
            options.usdExchangeRate,
            options.selectedGroup,
            false,
            extraMultiplier
          ),
          originalValue: hasDiscount
            ? formatPrice(
                props.model,
                'input',
                tokenUnit,
                options.showRechargePrice,
                options.priceRate,
                options.usdExchangeRate,
                options.selectedGroup,
                false
              )
            : undefined,
        },
        {
          label: t('Output'),
          value: formatPrice(
            model,
            'output',
            tokenUnit,
            options.showRechargePrice,
            options.priceRate,
            options.usdExchangeRate,
            options.selectedGroup,
            false,
            extraMultiplier
          ),
          originalValue: hasDiscount
            ? formatPrice(
                props.model,
                'output',
                tokenUnit,
                options.showRechargePrice,
                options.priceRate,
                options.usdExchangeRate,
                options.selectedGroup,
                false
              )
            : undefined,
        },
      ]
    } else {
      metrics = [
        {
          label: t('Per-request'),
          value: formatRequestPrice(
            model,
            options.showRechargePrice,
            options.priceRate,
            options.usdExchangeRate,
            options.selectedGroup,
            false,
            extraMultiplier
          ),
          originalValue: hasDiscount
            ? formatRequestPrice(
                props.model,
                options.showRechargePrice,
                options.priceRate,
                options.usdExchangeRate,
                options.selectedGroup,
                false
              )
            : undefined,
        },
      ]
      caption = `${currencyLabel} / ${t('request')}`
    }
  }
  return (
    <span className='block w-full max-w-full min-w-0 space-y-1.5'>
      <span
        className={metrics.length > 1 ? 'grid grid-cols-2 gap-x-4' : 'grid'}
      >
        {metrics.map((metric) => (
          <span
            key={metric.label}
            className='flex min-w-0 flex-col items-start gap-y-0.5 sm:flex-row sm:flex-wrap sm:items-baseline sm:gap-x-1.5'
          >
            <span
              className='text-muted-foreground min-w-0 truncate text-xs font-normal'
              title={metric.label}
            >
              {metric.label}
            </span>
            <span
              className='min-w-0 font-mono text-sm break-words whitespace-normal tabular-nums'
              title={metric.value}
            >
              {metric.value}
            </span>
            {metric.originalValue && (
              <span
                className='text-muted-foreground/60 min-w-0 font-mono text-xs break-words whitespace-normal tabular-nums line-through'
                title={t('Original price')}
              >
                {metric.originalValue}
              </span>
            )}
          </span>
        ))}
      </span>
      <span className='flex flex-wrap items-center gap-1.5'>
        {hasDiscount && props.model.discount_group && (
          <Badge variant='warning'>{props.model.discount_group}</Badge>
        )}
        <span
          className='text-muted-foreground block text-xs font-normal break-words whitespace-normal sm:truncate'
          title={caption}
        >
          {caption}
        </span>
      </span>
    </span>
  )
}
