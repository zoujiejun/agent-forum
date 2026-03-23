import React from 'react'
import { Button, Card, Input, List, Space, Tag, message } from 'antd'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { addTopicTags, closeTopic, createReply, getTopic, setTopicTags } from '../api'

function normalizeMarkdownContent(content: string) {
  return content
    .replace(/\\r\\n/g, '\n')
    .replace(/\\n/g, '\n')
}

function normalizeTagsInput(value: string) {
  return Array.from(
    new Set(
      value
        .split(',')
        .map(tag => tag.trim().toLowerCase())
        .filter(Boolean),
    ),
  )
}

export default function TopicDetail({ topicId, onReplied, refreshKey = 0 }: { topicId: number, onReplied: () => void, refreshKey?: number }) {
  const [topic, setTopic] = React.useState<any | null>(null)
  const [replyContent, setReplyContent] = React.useState('')
  const [tagInput, setTagInput] = React.useState('')
  const [submitting, setSubmitting] = React.useState(false)
  const [tagSaving, setTagSaving] = React.useState(false)
  const [closing, setClosing] = React.useState(false)

  const load = React.useCallback(async () => {
    const data = await getTopic(topicId)
    setTopic(data)
    setTagInput((data.tags || []).map((tag: any) => tag.name).join(', '))
  }, [topicId])

  React.useEffect(() => {
    load()
  }, [load, refreshKey])

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

  const saveTags = async () => {
    setTagSaving(true)
    try {
      await setTopicTags(topicId, normalizeTagsInput(tagInput))
      message.success('Tags updated')
      await load()
      onReplied()
    } catch (e: any) {
      message.error('Update tags failed: ' + (e?.response?.data?.error || e?.message || e))
    } finally {
      setTagSaving(false)
    }
  }

  const addQuickTag = async (tag: string) => {
    setTagSaving(true)
    try {
      await addTopicTags(topicId, [tag])
      await load()
      onReplied()
    } catch (e: any) {
      message.error('Add tag failed: ' + (e?.response?.data?.error || e?.message || e))
    } finally {
      setTagSaving(false)
    }
  }

  const doCloseTopic = async () => {
    setClosing(true)
    try {
      await closeTopic(topicId)
      message.success('Topic closed')
      await load()
      onReplied()
    } catch (e: any) {
      message.error('Close topic failed: ' + (e?.response?.data?.error || e?.message || e))
    } finally {
      setClosing(false)
    }
  }

  if (!topic) return <Card className="topic-detail-card">Loading...</Card>

  return (
    <Card
      className="topic-detail-card"
      title={
        <Space wrap>
          <span className="topic-title">[{topic.id}] {topic.title}</span>
          <Tag>{topic.creator?.name}</Tag>
          <Tag color={topic.status === 'closed' ? 'red' : 'green'}>{topic.status}</Tag>
        </Space>
      }
      extra={
        <Button danger onClick={doCloseTopic} loading={closing} disabled={topic.status === 'closed'}>
          Close Topic
        </Button>
      }
    >
      <div style={{ marginBottom: 12 }}>
        <Space wrap>
          {(topic.tags || []).length > 0 ? topic.tags.map((tag: any) => (
            <Tag key={tag.id ?? tag.name} color="blue">{tag.name}</Tag>
          )) : <Tag>no tags</Tag>}
        </Space>
      </div>

      <div style={{ marginBottom: 16 }}>
        <Input
          value={tagInput}
          onChange={e => setTagInput(e.target.value)}
          placeholder="Tags, separated by commas"
          addonAfter={<Button type="link" onClick={saveTags} loading={tagSaving}>Save Tags</Button>}
        />
        <Space wrap style={{ marginTop: 8 }}>
          {['p0', 'p1', 'bug', 'task', 'review', 'blocked'].map((tag) => (
            <Button key={tag} size="small" onClick={() => addQuickTag(tag)} disabled={tagSaving}>
              + {tag}
            </Button>
          ))}
        </Space>
      </div>

      <div className="markdown-body" style={{ marginBottom: 16 }}>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{normalizeMarkdownContent(topic.content)}</ReactMarkdown>
      </div>

      <List
        className="reply-list"
        header={<b>Replies ({topic.replies?.length || 0})</b>}
        dataSource={topic.replies || []}
        locale={{ emptyText: 'No replies yet' }}
        renderItem={(reply: any) => (
          <List.Item style={{ flexDirection: 'column', alignItems: 'flex-start' }}>
            <div style={{ fontSize: 13, color: '#666', marginBottom: 6 }}>
              {reply.author?.name} · {reply.created_at ? new Date(reply.created_at).toLocaleString('en-US') : ''}
            </div>
            <div className="markdown-body" style={{ width: '100%' }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{normalizeMarkdownContent(reply.content)}</ReactMarkdown>
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
        disabled={topic.status === 'closed'}
      />
      <div style={{ textAlign: 'right' }}>
        <Button type="primary" onClick={doReply} loading={submitting} disabled={!replyContent.trim() || topic.status === 'closed'}>
          Post Reply
        </Button>
      </div>
    </Card>
  )
}
