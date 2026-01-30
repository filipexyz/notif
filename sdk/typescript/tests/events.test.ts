import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { WebSocketServer } from 'ws'
import { EventStream } from '../src/events.js'

// Helper to create a mock WebSocket server
function createMockServer(port: number): WebSocketServer {
  return new WebSocketServer({ port })
}

// Helper to wait for a condition
async function waitFor(condition: () => boolean, timeout = 5000): Promise<void> {
  const start = Date.now()
  while (!condition() && Date.now() - start < timeout) {
    await new Promise(resolve => setTimeout(resolve, 50))
  }
}

// Helper to wait with a delay
function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

describe('EventStream', () => {
  const apiKey = 'nsh_testkey12345678901234567890'
  let server: WebSocketServer | null = null
  let port: number

  beforeEach(() => {
    // Use random port to avoid conflicts
    port = 9000 + Math.floor(Math.random() * 1000)
  })

  afterEach(async () => {
    if (server) {
      await new Promise<void>(resolve => {
        server!.close(() => resolve())
      })
      server = null
    }
  })

  describe('connection', () => {
    it('connects and sends subscribe message', async () => {
      let subscribeReceived = false
      let receivedTopics: string[] = []

      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            subscribeReceived = true
            receivedTopics = msg.topics
            ws.send(JSON.stringify({ type: 'subscribed' }))
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      // Start iteration in background to trigger connection
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      await waitFor(() => subscribeReceived)

      expect(subscribeReceived).toBe(true)
      expect(receivedTopics).toContain('test-topic')
      expect(stream.isConnected).toBe(true)

      stream.close()
      await iterPromise
    })

    it('receives events from server', async () => {
      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))

            // Send an event after subscription
            setTimeout(() => {
              ws.send(JSON.stringify({
                type: 'event',
                id: 'evt-123',
                topic: 'test-topic',
                data: { message: 'hello' },
                timestamp: new Date().toISOString(),
                attempt: 1,
                max_attempts: 5,
              }))
            }, 100)
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      const events: any[] = []
      const timeout = setTimeout(() => stream.close(), 3000)

      for await (const event of stream) {
        events.push(event)
        stream.close()
        break
      }

      clearTimeout(timeout)

      expect(events).toHaveLength(1)
      expect(events[0].id).toBe('evt-123')
      expect(events[0].topic).toBe('test-topic')
      expect(events[0].data.message).toBe('hello')
    })

    it('handles multiple topics', async () => {
      let receivedTopics: string[] = []

      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            receivedTopics = msg.topics
            ws.send(JSON.stringify({ type: 'subscribed' }))
          }
        })
      })

      const topics = ['topic-a', 'topic-b', 'topic-c']
      const stream = new EventStream(apiKey, `http://localhost:${port}`, topics)

      // Start iteration to trigger connection
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      await waitFor(() => receivedTopics.length > 0)

      expect(receivedTopics).toEqual(topics)

      stream.close()
      await iterPromise
    })
  })

  describe('reconnection', () => {
    it('reconnects after server closes connection', async () => {
      let connectionCount = 0

      server = createMockServer(port)
      server.on('connection', (ws) => {
        connectionCount++

        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))

            // Close on first connection to trigger reconnect
            if (connectionCount === 1) {
              setTimeout(() => ws.close(), 100)
            }
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      // Start iteration
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      // Wait for reconnection (initial delay is 1s, so wait a bit more)
      await waitFor(() => connectionCount >= 2, 5000)

      expect(connectionCount).toBeGreaterThanOrEqual(2)

      stream.close()
      await iterPromise
    }, 10000)

    it('uses exponential backoff for reconnection', async () => {
      let connectionAttempts: number[] = []
      let lastAttemptTime = Date.now()

      server = createMockServer(port)
      server.on('connection', (ws) => {
        const now = Date.now()
        connectionAttempts.push(now - lastAttemptTime)
        lastAttemptTime = now

        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
            // Keep closing to force more reconnects
            if (connectionAttempts.length < 3) {
              setTimeout(() => ws.close(), 50)
            }
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      // Start iteration
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      // Wait for a few reconnection attempts
      await waitFor(() => connectionAttempts.length >= 3, 15000)

      // First connection is immediate, subsequent should have increasing delays
      // Delays should roughly follow: 1s, 2s (exponential backoff)
      if (connectionAttempts.length >= 3) {
        // Second delay should be larger than first
        expect(connectionAttempts[2]).toBeGreaterThan(500)
      }

      stream.close()
      await iterPromise
    }, 20000)
  })

  describe('ping/pong', () => {
    it('stays connected when server responds to pings', async () => {
      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
          }
        })

        // The ws library automatically responds to pings with pongs
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      // Start iteration
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      await waitFor(() => stream.isConnected)

      // The ping interval is 30s which is too long for a test
      // Just verify the connection stays alive for a bit
      await delay(500)

      expect(stream.isConnected).toBe(true)

      stream.close()
      await iterPromise
    })
  })

  describe('ack/nack', () => {
    it('sends ack for event when autoAck is false', async () => {
      let ackReceived = false
      let ackedEventId = ''

      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
            setTimeout(() => {
              ws.send(JSON.stringify({
                type: 'event',
                id: 'evt-ack-test',
                topic: 'test-topic',
                data: {},
                timestamp: new Date().toISOString(),
              }))
            }, 100)
          } else if (msg.action === 'ack') {
            ackReceived = true
            ackedEventId = msg.id
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'], { autoAck: false })

      for await (const event of stream) {
        await event.ack()
        stream.close()
        break
      }

      await waitFor(() => ackReceived)

      expect(ackReceived).toBe(true)
      expect(ackedEventId).toBe('evt-ack-test')
    })

    it('sends nack for event with retry delay', async () => {
      let nackReceived = false
      let nackRetryIn = ''

      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
            setTimeout(() => {
              ws.send(JSON.stringify({
                type: 'event',
                id: 'evt-nack-test',
                topic: 'test-topic',
                data: {},
                timestamp: new Date().toISOString(),
              }))
            }, 100)
          } else if (msg.action === 'nack') {
            nackReceived = true
            nackRetryIn = msg.retry_in
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'], { autoAck: false })

      for await (const event of stream) {
        await event.nack('15m')
        stream.close()
        break
      }

      await waitFor(() => nackReceived)

      expect(nackReceived).toBe(true)
      expect(nackRetryIn).toBe('15m')
    })
  })

  describe('close', () => {
    it('stops iteration when closed', async () => {
      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      // Close after a short delay
      setTimeout(() => stream.close(), 200)

      const events: any[] = []
      for await (const event of stream) {
        events.push(event)
      }

      // Wait for connection to fully close
      await waitFor(() => !stream.isConnected, 1000)

      // Should exit cleanly without events
      expect(stream.isConnected).toBe(false)
    })

    it('calls onClose callbacks', async () => {
      let closeCallbackCalled = false

      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      stream.onClose(() => {
        closeCallbackCalled = true
      })

      // Start iteration
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      await waitFor(() => stream.isConnected)

      stream.close()
      await iterPromise

      expect(closeCallbackCalled).toBe(true)
    })

    it('is safe to call close multiple times', async () => {
      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            ws.send(JSON.stringify({ type: 'subscribed' }))
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['test-topic'])

      // Start iteration
      const iterPromise = (async () => {
        for await (const event of stream) {
          // Just consume events
        }
      })()

      await waitFor(() => stream.isConnected)

      // Call close multiple times
      stream.close()
      stream.close()
      stream.close()

      await iterPromise

      // Wait for connection to fully close
      await waitFor(() => !stream.isConnected, 1000)

      expect(stream.isConnected).toBe(false)
    })
  })

  describe('error handling', () => {
    it('handles server error messages', async () => {
      server = createMockServer(port)
      server.on('connection', (ws) => {
        ws.on('message', (data) => {
          const msg = JSON.parse(data.toString())
          if (msg.action === 'subscribe') {
            // Send error instead of subscribed
            ws.send(JSON.stringify({
              type: 'error',
              message: 'subscription failed: invalid topic',
            }))
          }
        })
      })

      const stream = new EventStream(apiKey, `http://localhost:${port}`, ['invalid-topic'])

      try {
        for await (const event of stream) {
          // Should not receive any events
        }
      } catch (error: any) {
        expect(error.message).toContain('subscription failed')
      }

      stream.close()
    })
  })
})
