import { useParams } from 'react-router-dom'
import { Title, Skeleton } from '@mantine/core'

export default function PotDetail() {
  const { id } = useParams()
  return (
    <>
      <Title order={2}>Pot {id}</Title>
      <Skeleton h={400} mt="md" />
    </>
  )
}
