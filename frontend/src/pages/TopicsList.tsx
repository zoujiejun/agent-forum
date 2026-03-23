import React from 'react'
import { Card, Input, List, Spin, Tag } from 'antd'
import { listTopics } from '../api'

function toPreviewText(content: string, maxLen = 100) {
  const plain = (content || '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[>#*_~\-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

  if (plain.length <= maxLen) {
    return plain
  }
  return `${plain.slice(0, maxLen).trim()}…`
}

export default function TopicsList({ onOpen, refreshKey }: { onOpen: (id: number) => void, refreshKey: number }) {
  const [topics, setTopics] = React.useState<any[]>([])
  const [loading, setLoading] = React.useState(false)
  const [keyword, setKeyword] = React.useState('')
  const [page, setPage] = React.useState(1)
  const [total, setTotal] = React.useState(0)
  const pageSize = 20

  const load = React.useCallback((pageNum = 1) => {
    setLoading(true)
    listTopics(pageNum, pageSize)
      .then((data) => {
        setTopics(data.topics)
        setTotal(data.total)
      })
      .finally(() => setLoading(false))
  }, [])

  React.useEffect(() => {
    load(page)
  }, [load, refreshKey, page])

  const filtered = topics.filter((topic) => {
    const q = keyword.trim().toLowerCase()
    if (!q) return true
    return `${topic.title} ${topic.content} ${topic.creator?.name || ''}`.toLowerCase().includes(q)
  })

  return (
    <Card title="Open Topics" className="topics-list-card">
      <Input
        placeholder="Search title/content/creator"
        value={keyword}
        onChange={e => setKeyword(e.target.value)}
        style={{ marginBottom: 12 }}
        allowClear
      />
      {loading ? <Spin /> : (
        <List
          dataSource={filtered}
          locale={{ emptyText: 'No topics' }}
          pagination={{
            current: page,
            pageSize,
            total,
            onChange: (p) => {
              setPage(p)
              window.scrollTo({ top: 0, behavior: 'smooth' })
            },
            showSizeChanger: false,
            showTotal: (count) => `Total ${count} topics`,
          }}
          renderItem={(topic: any) => (
            <List.Item
              onClick={() => onOpen(topic.id)}
              style={{ cursor: 'pointer' }}
              className="topic-list-item"
            >
              <List.Item.Meta
                title={
                  <span className="topic-item-title">
                    [#{topic.id}] {topic.title}
                    <Tag color="blue" style={{ marginLeft: 6, fontSize: 11 }}>
                      {topic.creator?.name || 'unknown'}
                    </Tag>
                  </span>
                }
                description={
                  <div style={{ color: '#666', fontSize: 12 }}>
                    {toPreviewText(topic.content)}
                  </div>
                }
              />
            </List.Item>
          )}
        />
      )}
    </Card>
  )
}
