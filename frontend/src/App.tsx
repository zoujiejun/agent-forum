import React from 'react'
import { Badge, Button, Drawer, Layout, Skeleton } from 'antd'
import { getNotifications } from './api'

const TopicsList = React.lazy(() => import('./pages/TopicsList'))
const TopicDetail = React.lazy(() => import('./pages/TopicDetail'))
const NewTopicModal = React.lazy(() => import('./pages/NewTopicModal'))
const SettingsPanel = React.lazy(() => import('./pages/SettingsPanel'))
const NotificationsPanel = React.lazy(() => import('./pages/NotificationsPanel'))

const { Header, Content, Sider } = Layout

function PanelFallback() {
  return <Skeleton active paragraph={{ rows: 3 }} title={{ width: '40%' }} />
}

export default function App() {
  const [selectedTopic, setSelectedTopic] = React.useState<number | null>(null)
  const [showNewTopic, setShowNewTopic] = React.useState(false)
  const [refreshKey, setRefreshKey] = React.useState(0)
  const [notifCount, setNotifCount] = React.useState(0)
  const [mobileView, setMobileView] = React.useState<'list' | 'detail'>('list')
  const [drawerOpen, setDrawerOpen] = React.useState(false)

  const refreshNotifications = React.useCallback(async () => {
    try {
      const data = await getNotifications()
      setNotifCount(Array.isArray(data) ? data.length : 0)
    } catch {
      setNotifCount(0)
    }
  }, [])

  React.useEffect(() => {
    refreshNotifications()
  }, [refreshNotifications, refreshKey])

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
        <React.Suspense fallback={<PanelFallback />}>
          <SettingsPanel onChanged={() => { setRefreshKey(v => v + 1); setDrawerOpen(false) }} />
          <div style={{ height: 12 }} />
          <NotificationsPanel refreshKey={refreshKey} onChanged={() => { setRefreshKey(v => v + 1) }} />
        </React.Suspense>
      </Drawer>

      <Layout className="desktop-layout">
        <Sider width={280} theme="light" className="desktop-sider">
          <React.Suspense fallback={<PanelFallback />}>
            <SettingsPanel onChanged={() => setRefreshKey(v => v + 1)} />
            <div style={{ height: 12 }} />
            <NotificationsPanel refreshKey={refreshKey} onChanged={() => { setRefreshKey(v => v + 1) }} />
          </React.Suspense>
        </Sider>
        <Content className="desktop-content">
          <div className="desktop-split">
            <div className="topics-col">
              <React.Suspense fallback={<PanelFallback />}>
                <TopicsList refreshKey={refreshKey} onOpen={handleOpenTopic} />
              </React.Suspense>
            </div>
            <div className="detail-col">
              <React.Suspense fallback={<PanelFallback />}>
                {selectedTopic ? (
                  <TopicDetail topicId={selectedTopic} onReplied={handleReplied} refreshKey={refreshKey} />
                ) : (
                  <div className="empty-hint">Select a topic to view details</div>
                )}
              </React.Suspense>
            </div>
          </div>
        </Content>
      </Layout>

      <div className="mobile-content">
        <React.Suspense fallback={<PanelFallback />}>
          {mobileView === 'list' ? (
            <TopicsList refreshKey={refreshKey} onOpen={handleOpenTopic} />
          ) : (
            selectedTopic ? (
              <TopicDetail topicId={selectedTopic} onReplied={handleReplied} refreshKey={refreshKey} />
            ) : null
          )}
        </React.Suspense>
      </div>

      <React.Suspense fallback={null}>
        <NewTopicModal
          open={showNewTopic}
          onCancel={() => setShowNewTopic(false)}
          onCreated={(id) => {
            setShowNewTopic(false)
            setSelectedTopic(id)
            setMobileView('detail')
            setRefreshKey(v => v + 1)
          }}
        />
      </React.Suspense>
    </Layout>
  )
}
