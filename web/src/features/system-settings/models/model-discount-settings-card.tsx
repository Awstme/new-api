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
import { toast } from 'sonner'

import { MultiSelect } from '@/components/multi-select'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { useModelPricing } from '@/features/model-pricing/api'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type ModelDiscountGroup = {
  id: string
  name: string
  discount: number
  models: string[]
  enabled: boolean
}

const OPTION_KEY = 'billing_setting.model_discount_groups'

function parseGroups(raw: string): ModelDiscountGroup[] {
  try {
    const parsed: unknown = JSON.parse(raw || '[]')
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (item): item is Record<string, unknown> =>
          typeof item === 'object' && item !== null
      )
      .map((item, index) => ({
        id:
          typeof item.id === 'string' && item.id.trim()
            ? item.id
            : `group-${index + 1}`,
        name: typeof item.name === 'string' ? item.name : '',
        discount: Number(item.discount) || 0,
        models: Array.isArray(item.models)
          ? item.models.filter(
              (model): model is string => typeof model === 'string'
            )
          : [],
        enabled: item.enabled !== false,
      }))
  } catch {
    return []
  }
}

function toPercent(discount: number): number {
  return Math.round(discount * 100)
}

interface Props {
  defaultValue: string
}

export function ModelDiscountSettingsCard(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const pricingQuery = useModelPricing()
  const [groups, setGroups] = useState<ModelDiscountGroup[]>(() =>
    parseGroups(props.defaultValue)
  )

  const modelOptions = useMemo(
    () =>
      (pricingQuery.data?.entries ?? [])
        .map((entry) => entry.model_name)
        .sort()
        .map((model) => ({ label: model, value: model })),
    [pricingQuery.data]
  )

  const updateGroup = (
    id: string,
    patch: Partial<Omit<ModelDiscountGroup, 'id'>>
  ) => {
    setGroups((current) =>
      current.map((group) => (group.id === id ? { ...group, ...patch } : group))
    )
  }

  const addGroup = () => {
    setGroups((current) => [
      ...current,
      {
        id: `group-${Date.now()}`,
        name: '',
        discount: 0.5,
        models: [],
        enabled: true,
      },
    ])
  }

  const removeGroup = (id: string) => {
    setGroups((current) => current.filter((group) => group.id !== id))
  }

  const validate = (): boolean => {
    const seen = new Map<string, string>()
    for (const group of groups) {
      if (!group.name.trim()) {
        toast.error(t('Discount group name is required'))
        return false
      }
      if (!(group.discount > 0 && group.discount <= 1)) {
        toast.error(t('Discount must be greater than 0 and at most 100'))
        return false
      }
      if (group.models.length === 0) {
        toast.error(t('Discount group must include at least one model'))
        return false
      }
      if (!group.enabled) continue
      for (const model of group.models) {
        const previous = seen.get(model)
        if (previous) {
          toast.error(
            t('Each model can belong to only one enabled discount group')
          )
          return false
        }
        seen.set(model, group.name)
      }
    }
    return true
  }

  const save = async () => {
    if (!validate()) return
    await updateOption.mutateAsync({
      key: OPTION_KEY,
      value: JSON.stringify(groups),
    })
  }

  return (
    <SettingsSection title={t('Model Discounts')}>
      <div className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Discount groups apply a promotional multiplier to selected models. The model square shows both the original and discounted prices.'
          )}
        </p>
        {groups.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('No discount groups configured')}
          </p>
        ) : (
          groups.map((group) => (
            <div
              key={group.id}
              className='border-border space-y-3 rounded-lg border p-4'
            >
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <Input
                  value={group.name}
                  onChange={(event) =>
                    updateGroup(group.id, { name: event.target.value })
                  }
                  placeholder={t('Group name')}
                  className='max-w-xs'
                />
                <div className='flex items-center gap-2'>
                  <span className='text-muted-foreground text-sm'>
                    {t('Enabled')}
                  </span>
                  <Switch
                    checked={group.enabled}
                    onCheckedChange={(checked) =>
                      updateGroup(group.id, { enabled: checked })
                    }
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    onClick={() => removeGroup(group.id)}
                  >
                    {t('Delete')}
                  </Button>
                </div>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Input
                  type='number'
                  min={1}
                  max={100}
                  step={1}
                  value={toPercent(group.discount)}
                  onChange={(event) =>
                    updateGroup(group.id, {
                      discount: Number(event.target.value) / 100,
                    })
                  }
                  className='max-w-28'
                  aria-label={t('Discount (%)')}
                />
                <span className='text-muted-foreground text-sm'>
                  {t('Discount (%)')}
                </span>
                <Badge variant='secondary'>{toPercent(group.discount)}%</Badge>
              </div>
              <div className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Models')}</span>
                <MultiSelect
                  options={modelOptions}
                  selected={group.models}
                  onChange={(models) => updateGroup(group.id, { models })}
                  placeholder={t('Select models')}
                  maxVisibleChips={5}
                />
              </div>
            </div>
          ))
        )}
        <Button type='button' variant='outline' size='sm' onClick={addGroup}>
          {t('Add discount group')}
        </Button>
      </div>
      <SettingsPageFormActions
        onSave={save}
        isSaving={updateOption.isPending}
        isSaveDisabled={pricingQuery.isLoading}
      />
    </SettingsSection>
  )
}
