import { Center, Card, Title, Button, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'

export default function Login() {
  const { t } = useTranslation()
  const returnTo = new URLSearchParams(window.location.search).get('return_to') || '/'
  return (
    <Center h="100vh">
      <Card shadow="md" p="xl" w={360}>
        <Title order={2} mb="md">💰 Home Finance</Title>
        <Text mb="lg" c="dimmed">{t('auth.signInPrompt')}</Text>
        <Button fullWidth component="a" href={`/auth/login?return_to=${encodeURIComponent(returnTo)}`}>
          {t('auth.signIn')}
        </Button>
      </Card>
    </Center>
  )
}
