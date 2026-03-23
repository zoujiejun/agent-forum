import React from 'react'
import { Card, Input, List, Spin, Tag } from 'antd'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { listTopics } from '../api'

export default function TopicsList({ onOpen, refreshKey }: { onOpen: (id: number) => void, refreshKey: number }) {
  const [topics, setTopics] = React.useState<any[]>([])
  const [loading, setLoading] = React.useState(false)
  const [keyword, setKeyword] = React.useState('')
  const [page, setPage] = React.useState(1)
  const [total, setTotal] = React.useState(0)
  const pageSize = 20

  const load = React.useCallback((pageNum = 1) => {
    setLoading(true)
    listTopics(pageNum, pageSize).then((data) => {
      setTopics(data?.topics || [])
      setTotal(data?.total || 0)
    }).finally(() => setLoading(false))
  }, [])

  React.useEffect(() => { load(page) }, [load, refreshKey, page])

  const filtered = topics.filter((t) => {
    const q = keyword.trim().toLowerCase()
    if (!q) return true
    return `${t.title} ${t.content} ${t.creator?.name || ''}`.toLowerCase().includes(q)
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
            onChange: (p) => { setPage(p); window.scrollTo({ top: 0, behavior: 'smooth' }) },
            showSizeChanger: false,
            showTotal: (t) => `Total ${t} topics`,
          }}
          renderItem={(t: any) => (
            <List.Item
              onClick={() => onOpen(t.id)}
              style={{ cursor: 'pointer' }}
              className="topic-list-item"
            >
              <List.Item.Meta
                title={
                  <span className="topic-item-title">
                    [#{t.id}] {t.title}
                    <Tag color="blue" style={{ marginLeft: 6, fontSize: 11 }}>{t.creator?.name || 'unknown'}</Tag>
                  </span>
                }
                description={
                  <div style={{ color: '#666', fontSize: 12 }} className="markdown-body">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {(t.content || '').slice(0, 80)}
                    </ReactMarkdown>
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
