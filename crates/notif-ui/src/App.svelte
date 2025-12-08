<script>
  import { invoke } from '@tauri-apps/api/core';
  import { onMount } from 'svelte';

  let notifications = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let activeTab = $state('pending'); // 'pending' or 'history'
  let statusFilter = $state('all'); // 'all', 'delivered', 'dismissed', 'approved'
  let expandedIds = $state(new Set()); // Track expanded notifications

  function toggleExpand(id) {
    if (expandedIds.has(id)) {
      expandedIds.delete(id);
    } else {
      expandedIds.add(id);
    }
    expandedIds = new Set(expandedIds); // Trigger reactivity
  }

  function estimateTokens(content) {
    if (!content) return 0;
    return Math.ceil(content.length / 4);
  }

  async function loadNotifications() {
    try {
      loading = true;
      error = null;
      if (activeTab === 'pending') {
        notifications = await invoke('get_pending');
      } else {
        const filter = statusFilter === 'all' ? null : statusFilter;
        notifications = await invoke('get_history', { statusFilter: filter, limit: 100 });
      }
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

  async function deleteNotification(id) {
    try {
      await invoke('delete_notif', { id });
      await loadNotifications();
    } catch (e) {
      error = e;
    }
  }

  function switchTab(tab) {
    activeTab = tab;
    loadNotifications();
  }

  function changeStatusFilter(e) {
    statusFilter = e.target.value;
    loadNotifications();
  }

  onMount(() => {
    loadNotifications();
    // Poll for new notifications every 2 seconds (only for pending tab)
    const interval = setInterval(() => {
      if (activeTab === 'pending') {
        loadNotifications();
      }
    }, 2000);
    return () => clearInterval(interval);
  });

  function getPriorityClass(priority) {
    switch (priority) {
      case 'high': return 'priority-high';
      case 'low': return 'priority-low';
      default: return 'priority-normal';
    }
  }

  function getStatusClass(status) {
    switch (status) {
      case 'approved': return 'status-approved';
      case 'dismissed': return 'status-dismissed';
      case 'delivered': return 'status-delivered';
      default: return 'status-pending';
    }
  }
</script>

<main>
  <header>
    <h1>notif.sh</h1>
    <span class="count">{notifications.length}</span>
  </header>

  <nav class="tabs">
    <button
      class="tab"
      class:active={activeTab === 'pending'}
      onclick={() => switchTab('pending')}
    >
      Pending
    </button>
    <button
      class="tab"
      class:active={activeTab === 'history'}
      onclick={() => switchTab('history')}
    >
      History
    </button>
  </nav>

  {#if activeTab === 'history'}
    <div class="filter-bar">
      <select value={statusFilter} onchange={changeStatusFilter}>
        <option value="all">All</option>
        <option value="delivered">Delivered</option>
        <option value="dismissed">Dismissed</option>
        <option value="approved">Approved</option>
        <option value="pending">Pending</option>
      </select>
    </div>
  {/if}

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if loading && notifications.length === 0}
    <div class="loading">Loading...</div>
  {:else if notifications.length === 0}
    <div class="empty">
      {#if activeTab === 'pending'}
        No pending notifications
      {:else}
        No notifications found
      {/if}
    </div>
  {:else}
    {#if activeTab === 'pending'}
      <div class="actions">
        <button class="approve-all" onclick={approveAll}>Approve All</button>
        <button class="dismiss-all" onclick={dismissAll}>Dismiss All</button>
      </div>
    {/if}

    <ul class="notifications">
      {#each notifications as notif (notif.id)}
        <li class="notification {getPriorityClass(notif.priority)}" class:expanded={expandedIds.has(notif.id)}>
          <div class="content-wrapper">
            <div class="content">
              <div class="meta">
                <span class="id">#{notif.id}</span>
                <span class="priority">{notif.priority}</span>
                <span class="status {getStatusClass(notif.status)}">{notif.status}</span>
                {#if notif.tags.length > 0}
                  <span class="tags">{notif.tags.join(', ')}</span>
                {/if}
              </div>
              <p class="message">{notif.message}</p>
              {#if notif.content}
                <button class="expand-btn" onclick={() => toggleExpand(notif.id)}>
                  {#if expandedIds.has(notif.id)}
                    ▼ Hide content
                  {:else}
                    ▶ Show content (~{estimateTokens(notif.content)} tokens)
                  {/if}
                </button>
                {#if expandedIds.has(notif.id)}
                  <div class="expanded-content">
                    <pre>{notif.content}</pre>
                  </div>
                {/if}
              {/if}
            </div>
            <div class="buttons">
              {#if activeTab === 'pending'}
                <button class="approve" onclick={() => approveNotification(notif.id)} title="Approve">
                  ✓
                </button>
                <button class="dismiss" onclick={() => dismissNotification(notif.id)} title="Dismiss">
                  ✗
                </button>
              {:else}
                <button class="delete" onclick={() => deleteNotification(notif.id)} title="Delete">
                  🗑
                </button>
              {/if}
            </div>
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
    margin-bottom: 0.5rem;
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

  .tabs {
    display: flex;
    gap: 0.25rem;
    margin-bottom: 0.75rem;
  }

  .tab {
    flex: 1;
    padding: 0.5rem;
    border: none;
    background: #2a2a4a;
    color: #888;
    cursor: pointer;
    border-radius: 0.25rem;
    font-size: 0.875rem;
    transition: all 0.15s;
  }

  .tab:hover {
    background: #3a3a5a;
    color: #aaa;
  }

  .tab.active {
    background: #4a4a6a;
    color: #fff;
  }

  .filter-bar {
    margin-bottom: 0.75rem;
  }

  .filter-bar select {
    width: 100%;
    padding: 0.5rem;
    background: #2a2a4a;
    color: #eee;
    border: 1px solid #4a4a6a;
    border-radius: 0.25rem;
    font-size: 0.875rem;
    cursor: pointer;
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
    margin-bottom: 0.75rem;
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
    flex-direction: column;
    background: #2a2a4a;
    border-radius: 0.5rem;
    margin-bottom: 0.5rem;
    overflow: hidden;
  }

  .content-wrapper {
    display: flex;
    align-items: stretch;
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
    flex-wrap: wrap;
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

  .status {
    text-transform: uppercase;
    font-weight: 600;
    padding: 0.1rem 0.3rem;
    border-radius: 0.2rem;
    font-size: 0.65rem;
  }

  .status-pending {
    background: #f59e0b33;
    color: #f59e0b;
  }

  .status-approved {
    background: #22c55e33;
    color: #22c55e;
  }

  .status-dismissed {
    background: #6b728033;
    color: #6b7280;
  }

  .status-delivered {
    background: #3b82f633;
    color: #3b82f6;
  }

  .tags {
    color: #a78bfa;
  }

  .message {
    margin: 0;
    font-size: 0.9375rem;
    line-height: 1.4;
  }

  .expand-btn {
    margin-top: 0.5rem;
    padding: 0.25rem 0.5rem;
    background: transparent;
    border: 1px solid #4a4a6a;
    color: #888;
    font-size: 0.75rem;
    border-radius: 0.25rem;
    cursor: pointer;
    transition: all 0.15s;
  }

  .expand-btn:hover {
    background: #3a3a5a;
    color: #aaa;
    border-color: #5a5a7a;
  }

  .expanded-content {
    margin-top: 0.5rem;
    padding: 0.75rem;
    background: #1a1a2e;
    border-radius: 0.25rem;
    border: 1px solid #3a3a5a;
  }

  .expanded-content pre {
    margin: 0;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
    font-size: 0.8125rem;
    line-height: 1.5;
    white-space: pre-wrap;
    word-wrap: break-word;
    color: #ccc;
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

  .delete {
    background: #dc2626;
    color: white;
    flex: 1;
  }

  .delete:hover {
    background: #b91c1c;
  }

  footer {
    margin-top: 0.75rem;
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
