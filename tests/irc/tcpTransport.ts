import net from 'node:net'
import type { IrcLineHandler, IrcTransport } from '../../app/irc/transport'

export type TcpIrcTransportOptions = {
  host: string
  port: number
}

export class TcpIrcTransport implements IrcTransport {
  private socket: net.Socket | null = null
  private lineHandler: IrcLineHandler = () => {}
  private closeHandler: () => void = () => {}
  private errorHandler: (error: Error) => void = () => {}
  private buffer = ''

  constructor(private readonly options: TcpIrcTransportOptions) {}

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const socket = net.createConnection(this.options.port, this.options.host)
      this.socket = socket

      socket.once('connect', () => resolve())
      socket.once('error', (error) => {
        this.errorHandler(error)
        reject(error)
      })

      socket.on('error', (error) => {
        this.errorHandler(error)
      })

      socket.on('close', () => {
        this.closeHandler()
      })

      socket.on('data', (chunk) => {
        this.buffer += chunk.toString('utf8')
        let index = this.buffer.indexOf('\n')
        while (index >= 0) {
          const line = this.buffer.slice(0, index).replace(/\r$/, '')
          this.buffer = this.buffer.slice(index + 1)
          if (line.length > 0) {
            this.lineHandler(line)
          }
          index = this.buffer.indexOf('\n')
        }
      })
    })
  }

  send(line: string): void {
    if (!this.socket || this.socket.destroyed) {
      throw new Error('TCP IRC transport is not connected')
    }

    this.socket.write(`${line}\r\n`)
  }

  close(): void {
    this.socket?.end()
    this.socket?.destroy()
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
