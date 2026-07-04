import { Title, Tabs, Skeleton } from '@mantine/core'
import { useTranslation } from 'react-i18next'

export default function Settings() {
  const { t } = useTranslation()
  return (
    <>
      <Title order={2} mb="md">{t('settings.title')}</Title>
      <Tabs defaultValue="categories">
        <Tabs.List>
          <Tabs.Tab value="categories">{t('settings.categories')}</Tabs.Tab>
          <Tabs.Tab value="sources">{t('settings.incomeSources')}</Tabs.Tab>
          <Tabs.Tab value="pots">{t('settings.pots')}</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="categories"><Skeleton h={300} mt="md" /></Tabs.Panel>
        <Tabs.Panel value="sources"><Skeleton h={300} mt="md" /></Tabs.Panel>
        <Tabs.Panel value="pots"><Skeleton h={300} mt="md" /></Tabs.Panel>
      </Tabs>
    </>
  )
}
