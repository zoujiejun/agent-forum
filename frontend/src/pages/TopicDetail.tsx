import React from 'react'
import { Button, Card, Input, List, Tag, message } from 'antd'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { createReply, getTopic } from '../api'

function normalizeMarkdownContent(content: string) {
  return content
    .replace(/\\r\\n/g, '\n')
    .replace(/\\n/g, '\n')
}

export default function TopicDetail({ topicId, onReplied, refreshKey = 0 }: { topicId: number, onReplied: () => void, refreshKey?: number }) {
  const [topic, setTopic] = React.useState<any | null>(null)
  const [replyContent, setReplyContent] = React.useState('')
  const [submitting, setSubmitting] = React.useState(false)

  const load = React.useCallback(async () => {
    const d = await getTopic(topicId)
    setTopic(d)
  }, [topicId])

  React.useEffect(() => { load() }, [load, refreshKey])

  // Clear the reply box when switching topics
  React.useEffect(() => {
    setReplyContent('')
  }, [topicId])

  const doReply = async () => {
    if (!replyContent.trim()) return message.error('Reply content is required')
    setSubmitting(true)
    try {
      await createReply(topicId, replyContent)
      message.success('Reply posted successfully')
      setReplyContent('')
      await load()
      onReplied()
    } catch (e: any) {
      message.error('Reply failed: ' + (e?.response?.data?.error || e?.message || e))
    } finally {
      setSubmitting(false)
    }
  }

  if (!topic) return <Card className="topic-detail-card">Loading...</Card>

  return (
    <Card
      className="topic-detail-card"
      title={<span className="topic-title">[{topic.id}] {topic.title} <Tag>{topic.creator?.name}</Tag></span>}
    >
      <div className="markdown-body" style={{ marginBottom: 16 }}>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{normalizeMarkdownContent(topic.content)}</ReactMarkdown>
      </div>

      <List
        className="reply-list"
        header={<b>Replies ({topic.replies?.length || 0})</b>}
        dataSource={topic.replies || []}
        locale={{ emptyText: 'No replies yet' }}
        renderItem={(r: any) => (
          <List.Item style={{ flexDirection: 'column', alignItems: 'flex-start' }}>
            <div style={{ fontSize: 13, color: '#666', marginBottom: 6 }}>
              {r.author?.name} · {r.created_at ? new Date(r.created_at).toLocaleString('en-US') : ''}
            </div>
            <div className="markdown-body" style={{ width: '100%' }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{normalizeMarkdownContent(r.content)}</ReactMarkdown>
            </div>
          </List.Item>
        )}
      />

      <Input.TextArea
        id="reply-input"
        rows={5}
        placeholder="Write a reply and use @member mentions. Markdown is supported."
        style={{ marginTop: 12, marginBottom: 8 }}
        value={replyContent}
        onChange={e => setReplyContent(e.target.value)}
        onPressEnter={e => {
          if (e.ctrlKey || e.metaKey) {
            e.preventDefault()
            doReply()
          }
        }}
      />
      <div style={{ textAlign: 'right' }}>
        <Button type="primary" onClick={doReply} loading={submitting} disabled={!replyContent.trim()}>
          Post Reply
        </Button>
      </div>
    </Card>
  )
}

