<script>
  import { invoke } from '@tauri-apps/api/core';
  import { onMount } from 'svelte';

  let notifications = $state([]);
  let loading = $state(true);
  let error = $state(null);

  async function loadNotifications() {
    try {
      loading = true;
      error = null;
      notifications = await invoke('get_pending');
    } catch (e) {
      error = e;
    } finally {
      loading = false;
    }
  }

  async function approveNotification(id) {
    try {
      await invoke('approve_notification', { id });
      await loadNotifications();
    } catch (e) {
      error = e;
    }
  }

  async function dismissNotification(id) {
    try {
      await invoke('dismiss_notification', { id });
      await loadNotifications();
    } catch (e) {
      error = e;
    }
  }

  async function approveAll() {
    try {
      await invoke('approve_all');
      await loadNotifications();
    } catch (e) {
      error = e;
    }
  }

  async function dismissAll() {
    try {
      await invoke('dismiss_all');
      await loadNotifications();
    } catch (e) {
      error = e;
    }
  }

  onMount(() => {
    loadNotifications();
    // Poll for new notifications every 2 seconds
    const interval = setInterval(loadNotifications, 2000);
    return () => clearInterval(interval);
  });

  function getPriorityClass(priority) {
    switch (priority) {
      case 'high': return 'priority-high';
      case 'low': return 'priority-low';
      default: return 'priority-normal';
    }
  }
</script>

<main>
  <header>
    <h1>notif</h1>
    <span class="count">{notifications.length}</span>
  </header>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if loading && notifications.length === 0}
    <div class="loading">Loading...</div>
  {:else if notifications.length === 0}
    <div class="empty">No pending notifications</div>
  {:else}
    <div class="actions">
      <button class="approve-all" onclick={approveAll}>Approve All</button>
      <button class="dismiss-all" onclick={dismissAll}>Dismiss All</button>
    </div>

    <ul class="notifications">
      {#each notifications as notif (notif.id)}
        <li class="notification {getPriorityClass(notif.priority)}">
          <div class="content">
            <div class="meta">
              <span class="id">#{notif.id}</span>
              <span class="priority">{notif.priority}</span>
              {#if notif.tags.length > 0}
                <span class="tags">{notif.tags.join(', ')}</span>
              {/if}
            </div>
            <p class="message">{notif.message}</p>
          </div>
          <div class="buttons">
            <button class="approve" onclick={() => approveNotification(notif.id)} title="Approve">
              ✓
            </button>
            <button class="dismiss" onclick={() => dismissNotification(notif.id)} title="Dismiss">
              ✗
            </button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}

  <footer>
    <button class="refresh" onclick={loadNotifications}>Refresh</button>
  </footer>
</main>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
    background: #1a1a2e;
    color: #eee;
  }

  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 1rem;
    box-sizing: border-box;
  }

  header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }

  h1 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 600;
  }

  .count {
    background: #4a4a6a;
    padding: 0.25rem 0.5rem;
    border-radius: 1rem;
    font-size: 0.875rem;
  }

  .error {
    background: #ff4444;
    color: white;
    padding: 0.5rem;
    border-radius: 0.25rem;
    margin-bottom: 1rem;
  }

  .loading, .empty {
    text-align: center;
    color: #888;
    padding: 2rem;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .actions {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }

  .actions button {
    flex: 1;
    padding: 0.5rem;
    border: none;
    border-radius: 0.25rem;
    cursor: pointer;
    font-size: 0.875rem;
  }

  .approve-all {
    background: #22c55e;
    color: white;
  }

  .dismiss-all {
    background: #6b7280;
    color: white;
  }

  .notifications {
    list-style: none;
    padding: 0;
    margin: 0;
    flex: 1;
    overflow-y: auto;
  }

  .notification {
    display: flex;
    align-items: stretch;
    background: #2a2a4a;
    border-radius: 0.5rem;
    margin-bottom: 0.5rem;
    overflow: hidden;
  }

  .notification.priority-high {
    border-left: 3px solid #ef4444;
  }

  .notification.priority-normal {
    border-left: 3px solid #f59e0b;
  }

  .notification.priority-low {
    border-left: 3px solid #6b7280;
  }

  .content {
    flex: 1;
    padding: 0.75rem;
  }

  .meta {
    display: flex;
    gap: 0.5rem;
    font-size: 0.75rem;
    color: #888;
    margin-bottom: 0.25rem;
  }

  .id {
    color: #60a5fa;
  }

  .priority {
    text-transform: uppercase;
    font-weight: 600;
  }

  .priority-high .priority {
    color: #ef4444;
  }

  .priority-normal .priority {
    color: #f59e0b;
  }

  .priority-low .priority {
    color: #6b7280;
  }

  .tags {
    color: #a78bfa;
  }

  .message {
    margin: 0;
    font-size: 0.9375rem;
    line-height: 1.4;
  }

  .buttons {
    display: flex;
    flex-direction: column;
  }

  .buttons button {
    border: none;
    padding: 0.75rem 1rem;
    cursor: pointer;
    font-size: 1rem;
    transition: background 0.15s;
  }

  .approve {
    background: #22c55e;
    color: white;
    flex: 1;
  }

  .approve:hover {
    background: #16a34a;
  }

  .dismiss {
    background: #6b7280;
    color: white;
    flex: 1;
  }

  .dismiss:hover {
    background: #4b5563;
  }

  footer {
    margin-top: 1rem;
  }

  .refresh {
    width: 100%;
    padding: 0.5rem;
    background: #4a4a6a;
    color: white;
    border: none;
    border-radius: 0.25rem;
    cursor: pointer;
  }

  .refresh:hover {
    background: #5a5a7a;
  }
</style>
