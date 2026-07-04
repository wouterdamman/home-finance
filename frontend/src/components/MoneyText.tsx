import { Text, TextProps } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { formatCents } from '../lib/money'

interface Props extends TextProps {
  cents: number
  colored?: boolean
}

export default function MoneyText({ cents, colored, ...props }: Props) {
  const { i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const color = colored ? (cents >= 0 ? 'green' : 'red') : undefined
  return <Text c={color} {...props}>{formatCents(cents, locale)}</Text>
}
