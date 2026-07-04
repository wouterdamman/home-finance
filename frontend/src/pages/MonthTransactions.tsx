import { useParams } from 'react-router-dom'
import { Title, Skeleton } from '@mantine/core'

export default function MonthTransactions() {
  const { year, month } = useParams()
  return (
    <>
      <Title order={2}>Transacties {year}/{month}</Title>
      <Skeleton h={600} mt="md" />
    </>
  )
}
