export type IrcLineHandler = (line: string) => void

export interface IrcTransport {
  connect(): Promise<void>
  send(line: string): void
  close(): void
  onLine(handler: IrcLineHandler): void
  onClose(handler: () => void): void
  onError(handler: (error: Error) => void): void
}

export type WebSocketIrcTransportOptions = {
  url: string
}

export class WebSocketIrcTransport implements IrcTransport {
  private socket: WebSocket | null = null
  private lineHandler: IrcLineHandler = () => {}
  private closeHandler: () => void = () => {}
  private errorHandler: (error: Error) => void = () => {}
  private buffer = ''

  constructor(private readonly options: WebSocketIrcTransportOptions) {}

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(this.options.url)
      this.socket = socket

      socket.onopen = () => resolve()
      socket.onerror = () => {
        const error = new Error(`Unable to connect to IRC websocket ${this.options.url}`)
        this.errorHandler(error)
        reject(error)
      }
      socket.onclose = () => {
        this.closeHandler()
      }
      socket.onmessage = (event) => {
        const chunk = typeof event.data === 'string' ? event.data : ''
        this.buffer += chunk

        let index = this.buffer.indexOf('\n')
        while (index >= 0) {
          const line = this.buffer.slice(0, index).replace(/\r$/, '')
          this.buffer = this.buffer.slice(index + 1)
          if (line.length > 0) {
            this.lineHandler(line)
          }
          index = this.buffer.indexOf('\n')
        }

        // WebSocket frames may not end with \r\n — flush remaining data as a complete line
        if (this.buffer.length > 0) {
          const remaining = this.buffer.replace(/\r$/, '')
          this.buffer = ''
          if (remaining.length > 0) {
            this.lineHandler(remaining)
          }
        }
      }
    })
  }

  send(line: string): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error('IRC websocket transport is not connected')
    }

    this.socket.send(`${line}\r\n`)
  }

  close(): void {
    this.socket?.close()
    this.socket = null
  }

  onLine(handler: IrcLineHandler): void {
    this.lineHandler = handler
  }

  onClose(handler: () => void): void {
    this.closeHandler = handler
  }

  onError(handler: (error: Error) => void): void {
    this.errorHandler = handler
  }
}
