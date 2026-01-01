interface LiveIndicatorProps {
  connected: boolean
  onClick?: () => void
}

export function LiveIndicator({ connected, onClick }: LiveIndicatorProps) {
  return (
    <button
      onClick={onClick}
      className="flex items-center gap-1.5 px-2 py-1 text-sm hover:bg-neutral-50"
    >
      <span
        className={`w-2 h-2 ${
          connected
            ? 'bg-error animate-pulse'
            : 'bg-neutral-300'
        }`}
        style={{ borderRadius: 0 }}
      />
      <span className={connected ? 'text-neutral-700' : 'text-neutral-500'}>
        {connected ? 'Live' : 'Paused'}
      </span>
    </button>
  )
}
