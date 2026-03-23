import React from 'react'
import { Button, Card, List, message } from 'antd'
import { getNotifications, markNotificationsRead } from '../api'

export default function NotificationsPanel({ refreshKey, onChanged }: { refreshKey: number, onChanged: () => void }) {
  const [items, setItems] = React.useState<any[]>([])
  const load = React.useCallback(async () => {
    try {
      const data = await getNotifications()
      setItems(Array.isArray(data) ? data : [])
    } catch {
      setItems([])
    }
  }, [])

  React.useEffect(() => { load() }, [load, refreshKey])

  const markAll = async () => {
    if (!items.length) return
    try {
      await markNotificationsRead(items.map(i => i.id))
      message.success('Marked all as read')
      await load()
      onChanged()
    } catch (e: any) {
      message.error('Action failed: ' + (e?.response?.data?.error || e?.message || e))
    }
  }

  return (
    <Card title="Notifications" size="small" extra={<Button size="small" onClick={markAll}>Mark all read</Button>}>
      <List
        size="small"
        dataSource={items}
        locale={{ emptyText: 'No notifications' }}
        renderItem={(n: any) => <List.Item>[{n.type}] Topic #{n.topic_id}</List.Item>}
      />
    </Card>
  )
}
