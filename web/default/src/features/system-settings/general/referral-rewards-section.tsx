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
import type { ChangeEvent } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ShieldAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const referralRewardsSchema = z.object({
  affiliate_setting: z.object({
    enabled: z.boolean(),
    attribution_window_days: z.coerce.number().int().min(0),
    min_topup_quota: z.coerce.number().int().min(0),
    settlement_delay_days: z.coerce.number().int().min(0),
    first_reward_enabled: z.boolean(),
    first_reward_rate: z.coerce.number().min(0),
    first_reward_cap_quota: z.coerce.number().int().min(0),
    recurring_reward_enabled: z.boolean(),
    recurring_reward_rate: z.coerce.number().min(0),
    recurring_window_days: z.coerce.number().int().min(0),
    recurring_max_count: z.coerce.number().int().min(0),
    recurring_cap_per_invitee: z.coerce.number().int().min(0),
    include_subscription_orders: z.boolean(),
  }),
})

type ReferralRewardsFormValues = z.infer<typeof referralRewardsSchema>
type NumberFieldName = keyof Pick<
  ReferralRewardsFormValues['affiliate_setting'],
  | 'attribution_window_days'
  | 'min_topup_quota'
  | 'settlement_delay_days'
  | 'first_reward_rate'
  | 'first_reward_cap_quota'
  | 'recurring_reward_rate'
  | 'recurring_window_days'
  | 'recurring_max_count'
  | 'recurring_cap_per_invitee'
>

type NumberFieldConfig = {
  name: NumberFieldName
  labelKey: string
  descriptionKey: string
  step?: string
  disabled?: boolean
}

type ReferralRewardsSectionProps = {
  defaultValues: ReferralRewardsFormValues
  complianceConfirmed?: boolean
}

const numberFieldGroups: Array<NumberFieldConfig[]> = [
  [
    {
      name: 'attribution_window_days',
      labelKey: 'Attribution window days',
      descriptionKey:
        'Recharge rewards only apply within this many days after registration. Use 0 for no limit.',
    },
    {
      name: 'min_topup_quota',
      labelKey: 'Minimum top-up quota',
      descriptionKey:
        'Top-ups below this actual credited quota will not trigger referral rewards.',
    },
    {
      name: 'settlement_delay_days',
      labelKey: 'Settlement delay days',
      descriptionKey:
        'Rewards stay pending for this many days before becoming transferable. Use 0 for immediate availability.',
    },
  ],
  [
    {
      name: 'first_reward_rate',
      labelKey: 'First reward rate',
      descriptionKey: 'Use decimal ratios, for example 0.1 means 10%.',
      step: '0.01',
    },
    {
      name: 'first_reward_cap_quota',
      labelKey: 'First reward cap quota',
      descriptionKey:
        'Maximum first top-up reward per invited user. Use 0 for no cap.',
    },
  ],
  [
    {
      name: 'recurring_reward_rate',
      labelKey: 'Recurring reward rate',
      descriptionKey: 'Use decimal ratios, for example 0.02 means 2%.',
      step: '0.01',
    },
    {
      name: 'recurring_window_days',
      labelKey: 'Recurring reward window days',
      descriptionKey:
        'Recurring rewards only apply within this many days after registration. Use 0 for no limit.',
    },
    {
      name: 'recurring_max_count',
      labelKey: 'Recurring reward count limit',
      descriptionKey:
        'Maximum recurring reward orders per invited user. Use 0 for no count limit.',
    },
    {
      name: 'recurring_cap_per_invitee',
      labelKey: 'Recurring reward cap per referral',
      descriptionKey:
        'Maximum total recurring reward quota per invited user. Use 0 for no cap.',
    },
  ],
]

export function ReferralRewardsSection({
  defaultValues,
  complianceConfirmed = true,
}: ReferralRewardsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const handleNumberChange =
    (onChange: (value: number | string) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      onChange(
        event.target.value === '' ? '' : event.currentTarget.valueAsNumber
      )
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<ReferralRewardsFormValues>({
      resolver: zodResolver(referralRewardsSchema) as Resolver<
        ReferralRewardsFormValues,
        unknown,
        ReferralRewardsFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })

  const rewardsEnabled = form.watch('affiliate_setting.enabled')
  const firstRewardsEnabled = form.watch(
    'affiliate_setting.first_reward_enabled'
  )
  const recurringRewardsEnabled = form.watch(
    'affiliate_setting.recurring_reward_enabled'
  )
  const controlsLocked = updateOption.isPending || isSubmitting
  const complianceLocked = !complianceConfirmed

  const renderNumberField = (config: NumberFieldConfig) => {
    const fieldName = `affiliate_setting.${config.name}` as const
    return (
      <FormField
        key={config.name}
        control={form.control}
        name={fieldName}
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t(config.labelKey)}</FormLabel>
            <FormControl>
              <Input
                type='number'
                min={0}
                step={config.step ?? '1'}
                value={field.value ?? ''}
                onChange={handleNumberChange(field.onChange)}
                name={field.name}
                onBlur={field.onBlur}
                ref={field.ref}
                disabled={config.disabled || controlsLocked}
              />
            </FormControl>
            <FormDescription>{t(config.descriptionKey)}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    )
  }

  return (
    <SettingsSection title={t('Referral Rewards')}>
      <FormNavigationGuard when={isDirty} />

      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <ShieldAlert data-icon='inline-start' />
          <AlertTitle>{t('Compliance confirmation required')}</AlertTitle>
          <AlertDescription>
            {t(
              'Referral rewards with non-zero rates require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />

          <FormField
            control={form.control}
            name='affiliate_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable referral rewards')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Registration only binds referral relationships. Rewards are created after successful top-ups.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={
                      controlsLocked || (complianceLocked && !field.value)
                    }
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <SettingsFormGrid>
            {numberFieldGroups[0].map(renderNumberField)}
          </SettingsFormGrid>

          <SettingsControlGroup>
            <FormField
              control={form.control}
              name='affiliate_setting.first_reward_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('First top-up rewards')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Apply a higher reward to the first successful top-up from each invited user.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={
                        controlsLocked ||
                        !rewardsEnabled ||
                        (complianceLocked && !field.value)
                      }
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <SettingsControlChildren>
              <SettingsFormGrid>
                {numberFieldGroups[1].map((field) =>
                  renderNumberField({
                    ...field,
                    disabled:
                      !rewardsEnabled ||
                      !firstRewardsEnabled ||
                      complianceLocked,
                  })
                )}
              </SettingsFormGrid>
            </SettingsControlChildren>
          </SettingsControlGroup>

          <SettingsControlGroup>
            <FormField
              control={form.control}
              name='affiliate_setting.recurring_reward_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Recurring rewards')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Apply lower long-term rewards to later eligible top-ups from invited users.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={
                        controlsLocked ||
                        !rewardsEnabled ||
                        (complianceLocked && !field.value)
                      }
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <SettingsControlChildren>
              <SettingsFormGrid>
                {numberFieldGroups[2].map((field) =>
                  renderNumberField({
                    ...field,
                    disabled:
                      !rewardsEnabled ||
                      !recurringRewardsEnabled ||
                      complianceLocked,
                  })
                )}
              </SettingsFormGrid>
            </SettingsControlChildren>
          </SettingsControlGroup>

          <FormField
            control={form.control}
            name='affiliate_setting.include_subscription_orders'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Include subscription orders')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Paid subscription orders can trigger referral rewards. Balance purchases are excluded.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={controlsLocked || !rewardsEnabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

export type { ReferralRewardsFormValues }
