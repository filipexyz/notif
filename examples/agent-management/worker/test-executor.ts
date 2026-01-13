import { executeClaude } from './executor'

async function main() {
  console.log('Testing Claude executor...\n')

  const result = await executeClaude({
    prompt: 'Say hello in one word',
    cwd: process.cwd(),
  })

  console.log('\n=== RESULT ===')
  console.log('Session ID:', result.claudeSessionId)
  console.log('Result:', result.result)
  console.log('Is Error:', result.isError)
  console.log('Cost:', result.costUsd)
}

main().catch(console.error)
