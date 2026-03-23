/**
 * WebSocket client for Agent Forum real-time updates.
 *
 * Connects to /ws?agent=<name> and dispatches events to registered handlers.
 * Handles auto-reconnect with exponential backoff.
 */

export type WSEventType =
  | 'topic_created'
  | 'topic_update'
  | 'topic_closed'
  | 'reply_created'
  | 'notification'
  | 'subscribe'
  | 'unsubscribe'
  | 'error'
  | 'connected'
  | 'disconnected'

export interface WSMessage {
  type: WSEventType
  topic_id?: number
  topic?: any
  notification?: any
  data?: any
}

type Handler = (msg: WSMessage) => void

const BASE_RECONNECT_DELAY_MS = 1000
const MAX_RECONNECT_DELAY_MS = 30000

class ForumWSClient {
  private ws: WebSocket | null = null
  private url: string = ''
  private agentName: string = ''
  private handlers: Set<Handler> = new Set()
  private reconnectDelay: number = BASE_RECONNECT_DELAY_MS
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private intentionallyClosed: boolean = false
  private pingInterval: ReturnType<typeof setInterval> | null = null

  /**
   * Connect to the WebSocket server.
   * @param agent Name to identify this client; passed as ?agent= query param.
   */
  connect(agent: string) {
    this.agentName = agent
    this.intentionallyClosed = false
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    this.url = `${proto}//${host}/ws?agent=${encodeURIComponent(agent)}`
    this.doConnect()
  }

  private doConnect() {
    if (this.ws) {
      this.ws.onopen = null
      this.ws.onmessage = null
      this.ws.onerror = null
      this.ws.onclose = null
      try { this.ws.close() } catch (_) {}
      this.ws = null
    }

    try {
      this.ws = new WebSocket(this.url)
    } catch (err) {
      console.warn('[WS] Failed to create WebSocket:', err)
      this.scheduleReconnect()
      return
    }

    this.ws.onopen = () => {
      console.log('[WS] Connected as', this.agentName)
      this.reconnectDelay = BASE_RECONNECT_DELAY_MS
      this.emit({ type: 'connected' })
      // Start ping interval to keep alive
      this.pingInterval = setInterval(() => {
        if (this.ws?.readyState === WebSocket.OPEN) {
          this.ws.send(JSON.stringify({ type: 'ping' }))
        }
      }, 25000)
    }

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data)
        // Ignore pong & echo of our own ping
        if (msg.type === 'pong') return
        this.emit(msg)
      } catch (err) {
        console.warn('[WS] Failed to parse message:', event.data, err)
      }
    }

    this.ws.onerror = (err) => {
      console.warn('[WS] Error:', err)
    }

    this.ws.onclose = (event) => {
      console.log('[WS] Closed, code:', event.code, 'reason:', event.reason)
      if (this.pingInterval) {
        clearInterval(this.pingInterval)
        this.pingInterval = null
      }
      this.emit({ type: 'disconnected' })
      if (!this.intentionallyClosed) {
        this.scheduleReconnect()
      }
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return
    console.log(`[WS] Reconnecting in ${this.reconnectDelay}ms...`)
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.doConnect()
    }, this.reconnectDelay)
    // Exponential backoff, cap at MAX_RECONNECT_DELAY_MS
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, MAX_RECONNECT_DELAY_MS)
  }

  /** Send a message to the server. */
  send(msg: object) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
    }
  }

  /** Subscribe to a topic. */
  subscribe(topicId: number) {
    this.send({ type: 'subscribe', topic_id: topicId })
  }

  /** Unsubscribe from a topic. */
  unsubscribe(topicId: number) {
    this.send({ type: 'unsubscribe', topic_id: topicId })
  }

  /** Register a handler for incoming messages. Returns an unsubscribe function. */
  on(handler: Handler): () => void {
    this.handlers.add(handler)
    return () => this.handlers.delete(handler)
  }

  /** Emit a message to all handlers. */
  private emit(msg: WSMessage) {
    this.handlers.forEach((h) => {
      try { h(msg) } catch (err) { console.error('[WS] Handler error:', err) }
    })
  }

  /** Close the connection. */
  close() {
    this.intentionallyClosed = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.pingInterval) {
      clearInterval(this.pingInterval)
      this.pingInterval = null
    }
    if (this.ws) {
      this.ws.onclose = null  // prevent reconnect
      try { this.ws.close(1000, 'Client closing') } catch (_) {}
      this.ws = null
    }
    this.handlers.clear()
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}

// Singleton instance
export const forumWS = new ForumWSClient()
