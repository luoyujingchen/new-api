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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import type { CustomOAuthProvider } from './custom-oauth/types'

const defaultSSOSchema = z.object({
  defaultProvider: z.string(),
})

type DefaultSSOFormValues = z.infer<typeof defaultSSOSchema>

export type DefaultSSOValues = {
  'sso.default_provider': string
  'oidc.enabled': boolean
  'oidc.client_id': string
  'oidc.authorization_endpoint': string
}

type DefaultSSOSectionProps = {
  defaultValues?: DefaultSSOValues
  customProviders?: CustomOAuthProvider[]
  customProvidersLoading?: boolean
}

type ProviderOption = {
  value: string
  label: string
  disabled?: boolean
}

const emptyDefaultSSOValues: DefaultSSOValues = {
  'sso.default_provider': '',
  'oidc.enabled': false,
  'oidc.client_id': '',
  'oidc.authorization_endpoint': '',
}

export function DefaultSSOSection({
  defaultValues,
  customProviders = [],
  customProvidersLoading = false,
}: DefaultSSOSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const resolvedDefaultValues = useMemo(
    () => ({
      ...emptyDefaultSSOValues,
      ...defaultValues,
    }),
    [defaultValues]
  )

  const normalizedDefaults: DefaultSSOFormValues = {
    defaultProvider: resolvedDefaultValues['sso.default_provider'] ?? '',
  }

  const form = useForm<DefaultSSOFormValues>({
    resolver: zodResolver(defaultSSOSchema),
    defaultValues: normalizedDefaults,
  })

  const providerOptions = useMemo<ProviderOption[]>(() => {
    const options: ProviderOption[] = [{ value: '', label: t('Disabled') }]

    if (
      resolvedDefaultValues['oidc.enabled'] &&
      resolvedDefaultValues['oidc.client_id'] &&
      resolvedDefaultValues['oidc.authorization_endpoint']
    ) {
      options.push({ value: 'oidc', label: t('OIDC') })
    }

    for (const provider of customProviders) {
      if (provider.enabled) {
        options.push({ value: provider.slug, label: provider.name })
      }
    }

    if (
      normalizedDefaults.defaultProvider &&
      !options.some(
        (option) => option.value === normalizedDefaults.defaultProvider
      )
    ) {
      options.push({
        value: normalizedDefaults.defaultProvider,
        label: t('Unavailable provider: {{provider}}', {
          provider: normalizedDefaults.defaultProvider,
        }),
        disabled: true,
      })
    }

    return options
  }, [
    customProviders,
    resolvedDefaultValues,
    normalizedDefaults.defaultProvider,
    t,
  ])

  const onSubmit = async (values: DefaultSSOFormValues) => {
    const selected = values.defaultProvider.trim()
    const selectedOption = providerOptions.find(
      (option) => option.value === selected
    )

    if (selected && (!selectedOption || selectedOption.disabled)) {
      toast.error(t('Selected SSO provider is unavailable'))
      return
    }

    if (selected === normalizedDefaults.defaultProvider) {
      toast.info(t('No changes to save'))
      return
    }

    const result = await updateOption.mutateAsync({
      key: 'sso.default_provider',
      value: selected,
    })
    if (!result.success) return

    form.reset({ defaultProvider: selected })
  }

  const handleReset = () => {
    form.reset(normalizedDefaults, {
      keepDirty: false,
      keepDirtyValues: false,
      keepErrors: false,
    })
    toast.success(t('Form reset to saved values'))
  }

  return (
    <>
      <FormNavigationGuard when={form.formState.isDirty} />

      <SettingsSection
        title={t('Default SSO')}
        description={t(
          'Automatically start this SSO provider when unauthenticated users open the sign-in page'
        )}
      >
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            <FormDirtyIndicator isDirty={form.formState.isDirty} />

            <FormField
              control={form.control}
              name='defaultProvider'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default SSO provider')}</FormLabel>
                  <FormControl>
                    <NativeSelect
                      className='w-full sm:w-80'
                      value={field.value}
                      onChange={field.onChange}
                      disabled={customProvidersLoading}
                    >
                      {providerOptions.map((option) => (
                        <NativeSelectOption
                          key={option.value === '' ? 'disabled' : option.value}
                          value={option.value}
                          disabled={option.disabled}
                        >
                          {option.label}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Choose Disabled to keep the normal sign-in page as the default'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='flex items-center gap-2'>
              <Button
                type='submit'
                disabled={!form.formState.isDirty || updateOption.isPending}
              >
                {t('Save')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={handleReset}
                disabled={!form.formState.isDirty || updateOption.isPending}
              >
                <RotateCcw className='h-4 w-4' />
                {t('Reset')}
              </Button>
            </div>
          </form>
        </Form>
      </SettingsSection>
    </>
  )
}
