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
import { useEffect } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const APPLICATION_HEADER_DETECTION_MODES = [
  'off',
  'observe',
  'enforce',
] as const

type ApplicationHeaderDetectionMode =
  (typeof APPLICATION_HEADER_DETECTION_MODES)[number]

const applicationHeaderDetectionSchema = z.object({
  ApplicationHeaderDetectionMode: z.enum(APPLICATION_HEADER_DETECTION_MODES),
})

type ApplicationHeaderDetectionFormValues = z.infer<
  typeof applicationHeaderDetectionSchema
>

type ApplicationHeaderDetectionSectionProps = {
  defaultValues: ApplicationHeaderDetectionFormValues
}

const MODE_DESCRIPTIONS: Record<ApplicationHeaderDetectionMode, string> = {
  off: 'Off: do not run Application Header detection.',
  observe: 'Observe: detect and log results without blocking requests.',
  enforce:
    'Enforce: block only when the bound application requires Header strict match.',
}

export function ApplicationHeaderDetectionSection({
  defaultValues,
}: ApplicationHeaderDetectionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<ApplicationHeaderDetectionFormValues>({
    resolver: zodResolver(applicationHeaderDetectionSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: ApplicationHeaderDetectionFormValues) => {
    if (
      values.ApplicationHeaderDetectionMode ===
      defaultValues.ApplicationHeaderDetectionMode
    ) {
      return
    }
    await updateOption.mutateAsync({
      key: 'ApplicationHeaderDetectionMode',
      value: values.ApplicationHeaderDetectionMode,
    })
  }

  return (
    <SettingsSection
      title={t('Application Header Detection')}
      description={t(
        'Identify request source by configured application Header rules and optionally enforce strict matching.'
      )}
    >
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className='flex flex-col gap-5'
        >
          <FormField
            control={form.control}
            name='ApplicationHeaderDetectionMode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Detection mode')}</FormLabel>
                <FormControl>
                  <ToggleGroup
                    value={[field.value]}
                    onValueChange={(value) => {
                      const nextValue = value[0]
                      if (nextValue) {
                        field.onChange(nextValue)
                      }
                    }}
                    spacing={2}
                    variant='outline'
                    aria-label={t('Application Header detection mode')}
                    className='flex-wrap'
                  >
                    {APPLICATION_HEADER_DETECTION_MODES.map((mode) => (
                      <ToggleGroupItem key={mode} value={mode}>
                        {t(mode)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </FormControl>
                <FormDescription>
                  {t(MODE_DESCRIPTIONS[field.value])}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='text-muted-foreground flex flex-col gap-1 text-sm'>
            <p>{t('off does not run Header application detection.')}</p>
            <p>
              {t('observe only records detection results in request logs.')}
            </p>
            <p>
              {t(
                'enforce applies blocking only to bound applications with Header strict match enabled.'
              )}
            </p>
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Button type='submit' disabled={updateOption.isPending}>
              {updateOption.isPending
                ? t('Saving...')
                : t('Save detection mode')}
            </Button>
            <Button
              variant='outline'
              render={
                <Link
                  to='/usage-logs/$section'
                  params={{ section: 'common' }}
                />
              }
            >
              {t('View request logs')}
            </Button>
          </div>
        </form>
      </Form>
    </SettingsSection>
  )
}
