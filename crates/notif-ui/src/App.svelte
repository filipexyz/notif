<script>
  import { invoke } from '@tauri-apps/api/core';
  import { onMount } from 'svelte';

  let notifications = $state([]);
  let remotes = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let activeTab = $state('pending'); // 'pending', 'history', or 'remotes'
  let statusFilter = $state('all');
  let expandedIds = $state(new Set());

  // Remote form state
  let showAddRemote = $state(false);
  let newRemote = $state({ name: '', url: '', apiKey: '', mode: 'store', tags: '', autoApprove: false });
  let testingRemote = $state(null);
  let pullingRemote = $state(null);
  let testResults = $state({});

  function toggleExpand(id) {
    if (expandedIds.has(id)) {
      expandedIds.delete(id);
    } else {
      expandedIds.add(id);
    }
    expandedIds = new Set(expandedIds);
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
      } else if (activeTab === 'history') {
        const filter = statusFilter === 'all' ? null : statusFilter;
        notifications = await invoke('get_history', { statusFilter: filter, limit: 100 });
      }
    } catch (e) {
      error = e;
    } finally {
      loading = false;
    }
  }

  async function loadRemotes() {
    try {
      loading = true;
      error = null;
      remotes = await invoke('get_remotes');
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

  async function addRemote() {
    try {
      const tags = newRemote.tags.split(',').map(t => t.trim()).filter(t => t);
      await invoke('add_remote_config', {
        name: newRemote.name,
        url: newRemote.url,
        apiKey: newRemote.apiKey,
        mode: newRemote.mode,
        tags,
        autoApprove: newRemote.autoApprove,
      });
      newRemote = { name: '', url: '', apiKey: '', mode: 'store', tags: '', autoApprove: false };
      showAddRemote = false;
      await loadRemotes();
    } catch (e) {
      error = e;
    }
  }

  async function removeRemote(name) {
    try {
      await invoke('remove_remote_config', { name });
      await loadRemotes();
    } catch (e) {
      error = e;
    }
  }

  async function testRemoteConnection(name) {
    testingRemote = name;
    testResults[name] = 'testing';
    try {
      await invoke('test_remote', { name });
      testResults[name] = 'success';
    } catch (e) {
      testResults[name] = 'error';
    } finally {
      testingRemote = null;
      setTimeout(() => {
        testResults[name] = null;
        testResults = { ...testResults };
      }, 3000);
    }
  }

  async function pullFromRemote(name) {
    pullingRemote = name;
    try {
      const count = await invoke('pull_remote', { name });
      await loadRemotes();
      if (count > 0) {
        error = null;
      }
    } catch (e) {
      error = e;
    } finally {
      pullingRemote = null;
    }
  }

  async function pullAllRemotes() {
    pullingRemote = 'all';
    try {
      await invoke('pull_all_remotes');
      await loadRemotes();
    } catch (e) {
      error = e;
    } finally {
      pullingRemote = null;
    }
  }

  function switchTab(tab) {
    activeTab = tab;
    if (tab === 'remotes') {
      loadRemotes();
    } else {
      loadNotifications();
    }
  }

  function changeStatusFilter(e) {
    statusFilter = e.target.value;
    loadNotifications();
  }

  function formatDate(dateStr) {
    if (!dateStr) return 'never';
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now - date;
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return date.toLocaleDateString();
  }

  onMount(() => {
    loadNotifications();
    const interval = setInterval(() => {
      if (activeTab === 'pending') {
        loadNotifications();
      } else if (activeTab === 'remotes') {
        loadRemotes();
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
    <span class="count">{activeTab === 'remotes' ? remotes.length : notifications.length}</span>
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
    <button
      class="tab tab-remote"
      class:active={activeTab === 'remotes'}
      onclick={() => switchTab('remotes')}
    >
      <span class="remote-icon">◉</span> Remotes
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

  {#if activeTab === 'remotes'}
    <!-- REMOTES PANEL -->
    <div class="remotes-panel">
      <div class="remotes-header">
        <div class="remotes-title">
          <span class="signal-icon">⦿</span>
          NETWORK ENDPOINTS
        </div>
        <div class="remotes-actions">
          {#if remotes.length > 0}
            <button
              class="pull-all-btn"
              onclick={pullAllRemotes}
              disabled={pullingRemote !== null}
            >
              {#if pullingRemote === 'all'}
                <span class="spinner"></span> SYNCING...
              {:else}
                ↻ SYNC ALL
              {/if}
            </button>
          {/if}
          <button
            class="add-remote-btn"
            onclick={() => showAddRemote = !showAddRemote}
          >
            {showAddRemote ? '✕ CANCEL' : '+ ADD REMOTE'}
          </button>
        </div>
      </div>

      {#if showAddRemote}
        <div class="add-remote-form">
          <div class="form-grid">
            <div class="form-field">
              <label>NAME</label>
              <input type="text" bind:value={newRemote.name} placeholder="ci-server" />
            </div>
            <div class="form-field">
              <label>URL</label>
              <input type="text" bind:value={newRemote.url} placeholder="https://server:8787" />
            </div>
            <div class="form-field">
              <label>API KEY</label>
              <input type="password" bind:value={newRemote.apiKey} placeholder="notif_..." />
            </div>
            <div class="form-field">
              <label>MODE</label>
              <select bind:value={newRemote.mode}>
                <option value="store">Store</option>
                <option value="passthrough">Passthrough</option>
              </select>
            </div>
            <div class="form-field">
              <label>TAGS (comma-separated)</label>
              <input type="text" bind:value={newRemote.tags} placeholder="ci, build" />
            </div>
            <div class="form-field checkbox-field">
              <label>
                <input type="checkbox" bind:checked={newRemote.autoApprove} />
                AUTO-APPROVE
              </label>
            </div>
          </div>
          <button class="submit-remote-btn" onclick={addRemote}>
            CONNECT REMOTE
          </button>
        </div>
      {/if}

      {#if loading && remotes.length === 0}
        <div class="loading">Scanning network...</div>
      {:else if remotes.length === 0}
        <div class="empty-remotes">
          <div class="empty-icon">◎</div>
          <p>No remote endpoints configured</p>
          <p class="empty-hint">Connect to remote notif servers to sync notifications</p>
        </div>
      {:else}
        <ul class="remotes-list">
          {#each remotes as remote (remote.name)}
            <li class="remote-card">
              <div class="remote-status-indicator" class:online={testResults[remote.name] !== 'error'}></div>
              <div class="remote-info">
                <div class="remote-name">{remote.name}</div>
                <div class="remote-url">{remote.url}</div>
                <div class="remote-meta">
                  <span class="remote-mode" class:passthrough={remote.mode === 'passthrough'}>
                    {remote.mode.toUpperCase()}
                  </span>
                  {#if remote.auto_approve}
                    <span class="remote-auto">AUTO</span>
                  {/if}
                  {#if remote.tags.length > 0}
                    <span class="remote-tags">{remote.tags.join(', ')}</span>
                  {/if}
                </div>
                <div class="remote-sync">
                  <span class="sync-label">LAST SYNC:</span>
                  <span class="sync-time">{formatDate(remote.last_synced_at)}</span>
                  <span class="sync-id">ID #{remote.last_synced_id}</span>
                </div>
              </div>
              <div class="remote-actions">
                <button
                  class="remote-btn test-btn"
                  class:success={testResults[remote.name] === 'success'}
                  class:error={testResults[remote.name] === 'error'}
                  onclick={() => testRemoteConnection(remote.name)}
                  disabled={testingRemote === remote.name}
                >
                  {#if testingRemote === remote.name}
                    ●●●
                  {:else if testResults[remote.name] === 'success'}
                    ✓ OK
                  {:else if testResults[remote.name] === 'error'}
                    ✕ ERR
                  {:else}
                    TEST
                  {/if}
                </button>
                <button
                  class="remote-btn pull-btn"
                  onclick={() => pullFromRemote(remote.name)}
                  disabled={pullingRemote === remote.name}
                >
                  {#if pullingRemote === remote.name}
                    <span class="spinner"></span>
                  {:else}
                    ↻ PULL
                  {/if}
                </button>
                <button
                  class="remote-btn remove-btn"
                  onclick={() => removeRemote(remote.name)}
                >
                  ✕
                </button>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {:else}
    <!-- NOTIFICATIONS PANEL -->
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
                  {#if notif.source_remote}
                    <span class="source-remote">@{notif.source_remote}</span>
                  {/if}
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
  {/if}

  <footer>
    <button class="refresh" onclick={() => activeTab === 'remotes' ? loadRemotes() : loadNotifications()}>Refresh</button>
  </footer>
</main>

<style>
  @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Outfit:wght@400;500;600;700&display=swap');

  :global(body) {
    margin: 0;
    padding: 0;
    font-family: 'Outfit', -apple-system, BlinkMacSystemFont, sans-serif;
    background: #0a0a12;
    color: #e0e0e8;
  }

  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 1rem;
    box-sizing: border-box;
    background:
      radial-gradient(ellipse at 20% 0%, rgba(16, 185, 129, 0.03) 0%, transparent 50%),
      radial-gradient(ellipse at 80% 100%, rgba(99, 102, 241, 0.03) 0%, transparent 50%),
      #0a0a12;
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
    background: linear-gradient(135deg, #10b981 0%, #6366f1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .count {
    background: rgba(99, 102, 241, 0.2);
    border: 1px solid rgba(99, 102, 241, 0.3);
    padding: 0.25rem 0.6rem;
    border-radius: 1rem;
    font-size: 0.8rem;
    font-weight: 500;
    color: #a5b4fc;
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
    background: rgba(30, 30, 50, 0.6);
    color: #6b7280;
    cursor: pointer;
    border-radius: 0.375rem;
    font-size: 0.875rem;
    font-weight: 500;
    transition: all 0.2s;
    border: 1px solid transparent;
  }

  .tab:hover {
    background: rgba(50, 50, 80, 0.6);
    color: #9ca3af;
  }

  .tab.active {
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
    border-color: rgba(99, 102, 241, 0.3);
  }

  .tab-remote {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
  }

  .tab-remote.active {
    background: rgba(16, 185, 129, 0.15);
    color: #6ee7b7;
    border-color: rgba(16, 185, 129, 0.3);
  }

  .remote-icon {
    font-size: 0.65rem;
    animation: pulse 2s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .filter-bar {
    margin-bottom: 0.75rem;
  }

  .filter-bar select {
    width: 100%;
    padding: 0.5rem;
    background: rgba(30, 30, 50, 0.8);
    color: #e0e0e8;
    border: 1px solid rgba(99, 102, 241, 0.2);
    border-radius: 0.375rem;
    font-size: 0.875rem;
    font-family: inherit;
    cursor: pointer;
  }

  .error {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #fca5a5;
    padding: 0.5rem 0.75rem;
    border-radius: 0.375rem;
    margin-bottom: 0.75rem;
    font-size: 0.875rem;
  }

  .loading, .empty {
    text-align: center;
    color: #6b7280;
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
    border-radius: 0.375rem;
    cursor: pointer;
    font-size: 0.875rem;
    font-weight: 500;
    font-family: inherit;
    transition: all 0.2s;
  }

  .approve-all {
    background: rgba(16, 185, 129, 0.2);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #6ee7b7;
  }

  .approve-all:hover {
    background: rgba(16, 185, 129, 0.3);
  }

  .dismiss-all {
    background: rgba(107, 114, 128, 0.2);
    border: 1px solid rgba(107, 114, 128, 0.3);
    color: #9ca3af;
  }

  .dismiss-all:hover {
    background: rgba(107, 114, 128, 0.3);
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
    background: rgba(20, 20, 35, 0.8);
    border-radius: 0.5rem;
    margin-bottom: 0.5rem;
    overflow: hidden;
    border: 1px solid rgba(99, 102, 241, 0.1);
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
    color: #6b7280;
    margin-bottom: 0.25rem;
    flex-wrap: wrap;
    align-items: center;
  }

  .id {
    color: #6366f1;
    font-family: 'JetBrains Mono', monospace;
  }

  .priority {
    text-transform: uppercase;
    font-weight: 600;
    font-size: 0.65rem;
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
    padding: 0.1rem 0.4rem;
    border-radius: 0.25rem;
    font-size: 0.6rem;
    letter-spacing: 0.02em;
  }

  .status-pending {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
  }

  .status-approved {
    background: rgba(16, 185, 129, 0.15);
    color: #6ee7b7;
  }

  .status-dismissed {
    background: rgba(107, 114, 128, 0.15);
    color: #9ca3af;
  }

  .status-delivered {
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
  }

  .source-remote {
    background: rgba(16, 185, 129, 0.15);
    color: #6ee7b7;
    padding: 0.1rem 0.4rem;
    border-radius: 0.25rem;
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.65rem;
  }

  .tags {
    color: #c4b5fd;
  }

  .message {
    margin: 0;
    font-size: 0.9375rem;
    line-height: 1.4;
    color: #e0e0e8;
  }

  .expand-btn {
    margin-top: 0.5rem;
    padding: 0.25rem 0.5rem;
    background: transparent;
    border: 1px solid rgba(99, 102, 241, 0.2);
    color: #6b7280;
    font-size: 0.75rem;
    border-radius: 0.25rem;
    cursor: pointer;
    transition: all 0.15s;
    font-family: inherit;
  }

  .expand-btn:hover {
    background: rgba(99, 102, 241, 0.1);
    color: #a5b4fc;
    border-color: rgba(99, 102, 241, 0.3);
  }

  .expanded-content {
    margin-top: 0.5rem;
    padding: 0.75rem;
    background: rgba(10, 10, 18, 0.8);
    border-radius: 0.375rem;
    border: 1px solid rgba(99, 102, 241, 0.1);
  }

  .expanded-content pre {
    margin: 0;
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.8125rem;
    line-height: 1.5;
    white-space: pre-wrap;
    word-wrap: break-word;
    color: #9ca3af;
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
    transition: all 0.15s;
  }

  .approve {
    background: rgba(16, 185, 129, 0.8);
    color: white;
    flex: 1;
  }

  .approve:hover {
    background: rgba(16, 185, 129, 1);
  }

  .dismiss {
    background: rgba(107, 114, 128, 0.8);
    color: white;
    flex: 1;
  }

  .dismiss:hover {
    background: rgba(107, 114, 128, 1);
  }

  .delete {
    background: rgba(239, 68, 68, 0.8);
    color: white;
    flex: 1;
  }

  .delete:hover {
    background: rgba(239, 68, 68, 1);
  }

  /* REMOTES PANEL STYLES */
  .remotes-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .remotes-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .remotes-title {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.75rem;
    font-weight: 600;
    color: #6ee7b7;
    letter-spacing: 0.1em;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .signal-icon {
    animation: pulse 1.5s infinite;
  }

  .remotes-actions {
    display: flex;
    gap: 0.5rem;
  }

  .add-remote-btn, .pull-all-btn {
    padding: 0.4rem 0.75rem;
    border: 1px solid rgba(16, 185, 129, 0.3);
    background: rgba(16, 185, 129, 0.1);
    color: #6ee7b7;
    border-radius: 0.375rem;
    font-size: 0.75rem;
    font-weight: 500;
    font-family: 'JetBrains Mono', monospace;
    cursor: pointer;
    transition: all 0.2s;
    letter-spacing: 0.02em;
  }

  .add-remote-btn:hover, .pull-all-btn:hover {
    background: rgba(16, 185, 129, 0.2);
    border-color: rgba(16, 185, 129, 0.5);
  }

  .pull-all-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .add-remote-form {
    background: rgba(16, 185, 129, 0.05);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: 0.5rem;
    padding: 1rem;
    margin-bottom: 0.75rem;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .form-field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .form-field label {
    font-size: 0.65rem;
    font-family: 'JetBrains Mono', monospace;
    color: #6ee7b7;
    letter-spacing: 0.1em;
    font-weight: 500;
  }

  .form-field input, .form-field select {
    padding: 0.5rem;
    background: rgba(10, 10, 18, 0.8);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: 0.375rem;
    color: #e0e0e8;
    font-size: 0.875rem;
    font-family: 'JetBrains Mono', monospace;
  }

  .form-field input:focus, .form-field select:focus {
    outline: none;
    border-color: rgba(16, 185, 129, 0.5);
    box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.1);
  }

  .form-field input::placeholder {
    color: #4b5563;
  }

  .checkbox-field {
    flex-direction: row;
    align-items: center;
  }

  .checkbox-field label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
  }

  .checkbox-field input[type="checkbox"] {
    width: 1rem;
    height: 1rem;
    accent-color: #10b981;
  }

  .submit-remote-btn {
    width: 100%;
    padding: 0.6rem;
    background: rgba(16, 185, 129, 0.2);
    border: 1px solid rgba(16, 185, 129, 0.4);
    color: #6ee7b7;
    border-radius: 0.375rem;
    font-size: 0.8rem;
    font-weight: 600;
    font-family: 'JetBrains Mono', monospace;
    cursor: pointer;
    transition: all 0.2s;
    letter-spacing: 0.05em;
  }

  .submit-remote-btn:hover {
    background: rgba(16, 185, 129, 0.3);
    border-color: rgba(16, 185, 129, 0.6);
  }

  .empty-remotes {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: #6b7280;
    text-align: center;
  }

  .empty-icon {
    font-size: 3rem;
    color: rgba(16, 185, 129, 0.2);
    margin-bottom: 0.5rem;
  }

  .empty-remotes p {
    margin: 0.25rem 0;
  }

  .empty-hint {
    font-size: 0.8rem;
    color: #4b5563;
  }

  .remotes-list {
    list-style: none;
    padding: 0;
    margin: 0;
    flex: 1;
    overflow-y: auto;
  }

  .remote-card {
    display: flex;
    align-items: stretch;
    background: rgba(16, 185, 129, 0.03);
    border: 1px solid rgba(16, 185, 129, 0.15);
    border-radius: 0.5rem;
    margin-bottom: 0.5rem;
    overflow: hidden;
    position: relative;
  }

  .remote-status-indicator {
    width: 4px;
    background: #6b7280;
    transition: background 0.3s;
  }

  .remote-status-indicator.online {
    background: #10b981;
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.5);
  }

  .remote-info {
    flex: 1;
    padding: 0.75rem;
    min-width: 0;
  }

  .remote-name {
    font-weight: 600;
    font-size: 0.95rem;
    color: #e0e0e8;
    margin-bottom: 0.15rem;
  }

  .remote-url {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.75rem;
    color: #6b7280;
    margin-bottom: 0.35rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .remote-meta {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
    margin-bottom: 0.35rem;
  }

  .remote-mode {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.6rem;
    padding: 0.15rem 0.4rem;
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
    border-radius: 0.25rem;
    font-weight: 500;
    letter-spacing: 0.02em;
  }

  .remote-mode.passthrough {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
  }

  .remote-auto {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.6rem;
    padding: 0.15rem 0.4rem;
    background: rgba(16, 185, 129, 0.15);
    color: #6ee7b7;
    border-radius: 0.25rem;
    font-weight: 500;
  }

  .remote-tags {
    font-size: 0.7rem;
    color: #c4b5fd;
  }

  .remote-sync {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.65rem;
    color: #4b5563;
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .sync-label {
    color: #3b4252;
  }

  .sync-time {
    color: #6b7280;
  }

  .sync-id {
    color: #4b5563;
  }

  .remote-actions {
    display: flex;
    flex-direction: column;
  }

  .remote-btn {
    border: none;
    padding: 0.5rem 0.75rem;
    font-size: 0.7rem;
    font-family: 'JetBrains Mono', monospace;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    letter-spacing: 0.02em;
  }

  .test-btn {
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
  }

  .test-btn:hover {
    background: rgba(99, 102, 241, 0.25);
  }

  .test-btn.success {
    background: rgba(16, 185, 129, 0.2);
    color: #6ee7b7;
  }

  .test-btn.error {
    background: rgba(239, 68, 68, 0.2);
    color: #fca5a5;
  }

  .pull-btn {
    background: rgba(16, 185, 129, 0.15);
    color: #6ee7b7;
  }

  .pull-btn:hover {
    background: rgba(16, 185, 129, 0.25);
  }

  .pull-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .remove-btn {
    background: rgba(239, 68, 68, 0.1);
    color: #f87171;
  }

  .remove-btn:hover {
    background: rgba(239, 68, 68, 0.2);
  }

  .spinner {
    display: inline-block;
    width: 0.75rem;
    height: 0.75rem;
    border: 2px solid transparent;
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  footer {
    margin-top: 0.75rem;
  }

  .refresh {
    width: 100%;
    padding: 0.5rem;
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.2);
    color: #a5b4fc;
    border-radius: 0.375rem;
    cursor: pointer;
    font-family: inherit;
    font-weight: 500;
    font-size: 0.875rem;
    transition: all 0.2s;
  }

  .refresh:hover {
    background: rgba(99, 102, 241, 0.25);
    border-color: rgba(99, 102, 241, 0.4);
  }
</style>
