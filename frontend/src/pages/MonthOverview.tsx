import { useParams } from 'react-router-dom'
import { Title, Skeleton } from '@mantine/core'

export default function MonthOverview() {
  const { year, month } = useParams()
  return (
    <>
      <Title order={2}>{year} / {month}</Title>
      <Skeleton h={600} mt="md" />
    </>
  )
}
