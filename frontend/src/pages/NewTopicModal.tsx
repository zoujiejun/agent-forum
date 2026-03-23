import React from 'react'
import { Button, Input, Modal, Select, message } from 'antd'
import { createTopic, listMembers } from '../api'

export default function NewTopicModal({ open, onCancel, onCreated }: {
  open: boolean
  onCancel: () => void
  onCreated: (id: number) => void
}) {
  const [title, setTitle] = React.useState('')
  const [content, setContent] = React.useState('')
  const [mentions, setMentions] = React.useState<string[]>([])
  const [memberOptions, setMemberOptions] = React.useState<{ label: string, value: string }[]>([])
  const [loading, setLoading] = React.useState(false)

  React.useEffect(() => {
    if (!open) return
    listMembers().then((data) => {
      setMemberOptions((data || []).map((m: any) => ({ label: m.name, value: m.name })))
    }).catch(() => setMemberOptions([]))
  }, [open])

  const submit = async () => {
    if (!title.trim() || !content.trim()) return message.error('Title and content are required')
    setLoading(true)
    try {
      const data = await createTopic(title, content, mentions)
      setTitle('')
      setContent('')
      setMentions([])
      onCreated(data.id)
    } catch (e: any) {
      message.error('Create failed: ' + (e?.response?.data?.error || e?.message || e))
    } finally {
      setLoading(false)
    }
  }

  const isMobile = typeof window !== 'undefined' && window.innerWidth < 768

  return (
    <Modal
      title="New Topic"
      open={open}
      onCancel={onCancel}
      footer={null}
      width={isMobile ? '100%' : 600}
      className={isMobile ? 'fullscreen-modal' : ''}
      bodyStyle={isMobile ? { padding: 12 } : {}}
      style={isMobile ? { top: 0, paddingBottom: 0 } : {}}
    >
      <Input
        placeholder="Title"
        value={title}
        onChange={e => setTitle(e.target.value)}
        style={{ marginBottom: 12 }}
        size="large"
      />
      <Input.TextArea
        rows={isMobile ? 8 : 6}
        placeholder="Content (Markdown supported)"
        value={content}
        onChange={e => setContent(e.target.value)}
        style={{ marginBottom: 12 }}
      />
      <Select
        mode="multiple"
        style={{ width: '100%', marginBottom: 12 }}
        placeholder="Select members to mention"
        options={memberOptions}
        value={mentions}
        onChange={setMentions}
        size="large"
      />
      <div style={{ textAlign: 'right' }}>
        <Button onClick={onCancel} style={{ marginRight: 8 }}>Cancel</Button>
        <Button type="primary" loading={loading} onClick={submit}>Create</Button>
      </div>
    </Modal>
  )
}
