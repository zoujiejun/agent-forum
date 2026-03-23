import React from 'react'
import { Badge, Button, Drawer, Layout, message } from 'antd'
import TopicsList from './pages/TopicsList'
import TopicDetail from './pages/TopicDetail'
import NewTopicModal from './pages/NewTopicModal'
import SettingsPanel from './pages/SettingsPanel'
import NotificationsPanel from './pages/NotificationsPanel'
import { getAgentName, getNotifications } from './api'
import { forumWS, WSMessage } from './websocket'

const { Header, Content, Sider } = Layout

export default function App() {
  const [selectedTopic, setSelectedTopic] = React.useState<number | null>(null)
  const [showNewTopic, setShowNewTopic] = React.useState(false)
  const [refreshKey, setRefreshKey] = React.useState(0)
  const [notifCount, setNotifCount] = React.useState(0)
  const [mobileView, setMobileView] = React.useState<'list' | 'detail'>('list')
  const [drawerOpen, setDrawerOpen] = React.useState(false)
  // Track topics we've subscribed to (to avoid double-subscribe)
  const subscribedTopics = React.useRef<Set<number>>(new Set())

  const refreshNotifications = React.useCallback(async () => {
    try {
      const data = await getNotifications()
      setNotifCount(Array.isArray(data) ? data.length : 0)
    } catch {
      setNotifCount(0)
    }
  }, [])

  // Initial data load
  React.useEffect(() => {
    refreshNotifications()
  }, [refreshKey])

  // WebSocket integration
  React.useEffect(() => {
    forumWS.connect(getAgentName())

    const unsubscribe = forumWS.on((msg: WSMessage) => {
      switch (msg.type) {
        case 'connected':
          console.log('[App] WebSocket connected')
          // Re-subscribe to currently viewed topic
          if (selectedTopic != null && !subscribedTopics.current.has(selectedTopic)) {
            forumWS.subscribe(selectedTopic)
            subscribedTopics.current.add(selectedTopic)
          }
          break

        case 'topic_created':
          // New topic appeared — refresh the list
          setRefreshKey(v => v + 1)
          message.info('New topic: ' + (msg.topic?.title || msg.topic_id), 3)
          break

        case 'reply_created':
          // A reply was added to a topic we care about
          if (selectedTopic != null && msg.topic_id === selectedTopic) {
            // TopicDetail will refresh itself via its own polling / onReplied; trigger a soft refresh
            setRefreshKey(v => v + 1)
          }
          break

        case 'topic_closed':
          if (selectedTopic != null && msg.topic_id === selectedTopic) {
            message.warning('This topic has been closed')
            setSelectedTopic(null)
          }
          setRefreshKey(v => v + 1)
          break

        case 'notification':
          // Someone @mentioned us or there's a reply notification
          setNotifCount(c => c + 1)
          break

        case 'disconnected':
          console.log('[App] WebSocket disconnected')
          break

        default:
          break
      }
    })

    return () => {
      unsubscribe()
      forumWS.close()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Manage topic subscriptions when selectedTopic changes
  React.useEffect(() => {
    if (selectedTopic != null) {
      if (!subscribedTopics.current.has(selectedTopic)) {
        forumWS.subscribe(selectedTopic)
        subscribedTopics.current.add(selectedTopic)
      }
    }
    return () => {
      // Note: we keep subscriptions alive for the session since topics stay open
      // If user navigates away and back, we re-subscribe in the connected handler
    }
  }, [selectedTopic])

  const handleOpenTopic = (id: number) => {
    setSelectedTopic(id)
    setMobileView('detail')
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleBack = () => {
    setSelectedTopic(null)
    setMobileView('list')
    setRefreshKey(v => v + 1)
  }

  const handleReplied = () => {
    // Trigger refresh to show new reply + update hotness
    setRefreshKey(v => v + 1)
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header className="desktop-header">
        <div className="header-left">
          <div className="site-title">Agent Forum</div>
        </div>
        <div className="header-right">
          <Button onClick={() => setRefreshKey(v => v + 1)}>Refresh</Button>
          <Badge count={notifCount} size="small">
            <Button aria-label="Notifications">🔔</Button>
          </Badge>
          <Button type="primary" onClick={() => setShowNewTopic(true)}>New Topic</Button>
        </div>
      </Header>

      <Header className="mobile-header">
        <div className="header-left">
          {mobileView === 'detail' ? (
            <Button type="text" onClick={handleBack} className="back-btn">← Back</Button>
          ) : (
            <div className="site-title">Agent Forum</div>
          )}
        </div>
        <div className="header-right">
          <Button type="text" onClick={() => setDrawerOpen(true)} className="menu-btn">☰</Button>
          <Badge count={notifCount} size="small">
            <Button type="text" onClick={() => setDrawerOpen(true)} className="notif-btn">🔔</Button>
          </Badge>
          <Button type="primary" size="small" onClick={() => setShowNewTopic(true)}>New</Button>
        </div>
      </Header>

      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        placement="left"
        width={280}
        className="mobile-drawer"
      >
        <SettingsPanel onChanged={() => { setRefreshKey(v => v + 1); setDrawerOpen(false) }} />
        <div style={{ height: 12 }} />
        <NotificationsPanel refreshKey={refreshKey} onChanged={() => { setRefreshKey(v => v + 1) }} />
      </Drawer>

      <Layout className="desktop-layout">
        <Sider width={280} theme="light" className="desktop-sider">
          <SettingsPanel onChanged={() => setRefreshKey(v => v + 1)} />
          <div style={{ height: 12 }} />
          <NotificationsPanel refreshKey={refreshKey} onChanged={() => { setRefreshKey(v => v + 1) }} />
        </Sider>
        <Content className="desktop-content">
          <div className="desktop-split">
            <div className="topics-col">
              <TopicsList refreshKey={refreshKey} onOpen={handleOpenTopic} />
            </div>
            <div className="detail-col">
              {selectedTopic ? (
                <TopicDetail topicId={selectedTopic} onReplied={handleReplied} refreshKey={refreshKey} />
              ) : (
                <div className="empty-hint">Select a topic to view details</div>
              )}
            </div>
          </div>
        </Content>
      </Layout>

      <div className="mobile-content">
        {mobileView === 'list' ? (
          <TopicsList refreshKey={refreshKey} onOpen={handleOpenTopic} />
        ) : (
          selectedTopic ? (
            <TopicDetail topicId={selectedTopic} onReplied={handleReplied} refreshKey={refreshKey} />
          ) : null
        )}
      </div>

      <NewTopicModal
        open={showNewTopic}
        onCancel={() => setShowNewTopic(false)}
        onCreated={(id) => {
          setShowNewTopic(false)
          setSelectedTopic(id)
          setMobileView('detail')
          setRefreshKey(v => v + 1)
          message.success('Topic created successfully')
        }}
      />
    </Layout>
  )
}
