// Shim for use-sync-external-store/shim/with-selector for React 19
import { useSyncExternalStore, useRef, useCallback } from 'react'

export function useSyncExternalStoreWithSelector<Snapshot, Selection>(
  subscribe: (onStoreChange: () => void) => () => void,
  getSnapshot: () => Snapshot,
  getServerSnapshot: (() => Snapshot) | undefined,
  selector: (snapshot: Snapshot) => Selection,
  isEqual?: (a: Selection, b: Selection) => boolean
): Selection {
  const instRef = useRef<{ hasValue: boolean; value: Selection } | null>(null)

  const getSelection = useCallback(() => {
    const nextSnapshot = getSnapshot()
    const nextSelection = selector(nextSnapshot)

    if (instRef.current !== null && isEqual !== undefined) {
      if (isEqual(instRef.current.value, nextSelection)) {
        return instRef.current.value
      }
    }

    instRef.current = { hasValue: true, value: nextSelection }
    return nextSelection
  }, [getSnapshot, selector, isEqual])

  const getServerSelection = getServerSnapshot === undefined
    ? undefined
    : () => selector(getServerSnapshot())

  return useSyncExternalStore(subscribe, getSelection, getServerSelection)
}

export default useSyncExternalStoreWithSelector
