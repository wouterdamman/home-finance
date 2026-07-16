import { Center, Card, Title, Button, Text, Group } from '@mantine/core'
import { IconWallet } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'

export default function Login() {
  const { t } = useTranslation()
  const returnTo = new URLSearchParams(window.location.search).get('return_to') || '/'
  return (
    <Center h="100vh">
      <Card shadow="md" p="xl" w={360}>
        <Group gap="xs" mb="md">
          <IconWallet size={24} />
          <Title order={2}>Home Finance</Title>
        </Group>
        <Text mb="lg" c="dimmed">{t('auth.signInPrompt')}</Text>
        <Button fullWidth component="a" href={`/auth/login?return_to=${encodeURIComponent(returnTo)}`}>
          {t('auth.signIn')}
        </Button>
      </Card>
    </Center>
  )
}
