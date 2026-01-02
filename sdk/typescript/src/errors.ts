export class NotifError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'NotifError'
  }
}

export class APIError extends NotifError {
  constructor(
    public readonly statusCode: number,
    message: string
  ) {
    super(`API error (${statusCode}): ${message}`)
    this.name = 'APIError'
  }
}

export class AuthError extends NotifError {
  constructor(message: string = 'invalid or missing API key') {
    super(`authentication error: ${message}`)
    this.name = 'AuthError'
  }
}

export class ConnectionError extends NotifError {
  constructor(
    message: string,
    public readonly cause?: Error
  ) {
    super(`connection error: ${message}`)
    this.name = 'ConnectionError'
  }
}
