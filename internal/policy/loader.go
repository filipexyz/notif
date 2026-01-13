package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Loader manages loading and hot-reloading of policy files
type Loader struct {
	policyDir string
	policies  map[string]*OrgPolicy // org_id -> policy
	mu        sync.RWMutex
	watcher   *fsnotify.Watcher
	stopChan  chan struct{}
}

// NewLoader creates a new policy loader
func NewLoader(policyDir string) (*Loader, error) {
	// Ensure policy directory exists
	if err := os.MkdirAll(policyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create policy directory: %w", err)
	}

	loader := &Loader{
		policyDir: policyDir,
		policies:  make(map[string]*OrgPolicy),
		stopChan:  make(chan struct{}),
	}

	// Load all existing policies
	if err := loader.loadAll(); err != nil {
		return nil, fmt.Errorf("failed to load policies: %w", err)
	}

	// Setup file watcher for hot reload
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	loader.watcher = watcher

	// Watch the policy directory
	if err := watcher.Add(policyDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch policy directory: %w", err)
	}

	// Start watching in background
	go loader.watch()

	return loader, nil
}

// loadAll loads all policy files from the directory
func (l *Loader) loadAll() error {
	entries, err := os.ReadDir(l.policyDir)
	if err != nil {
		return fmt.Errorf("failed to read policy directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}

		path := filepath.Join(l.policyDir, entry.Name())
		if err := l.loadFile(path); err != nil {
			// Log error but continue loading other files
			fmt.Printf("ERROR: Failed to load policy file %s: %v\n", path, err)
		}
	}

	return nil
}

// loadFile loads a single policy file
func (l *Loader) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var policy OrgPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate policy
	if err := l.validatePolicy(&policy); err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	// Store policy
	l.mu.Lock()
	l.policies[policy.OrgID] = &policy
	l.mu.Unlock()

	fmt.Printf("INFO: Loaded policy for org %s from %s (%d topics)\n",
		policy.OrgID, filepath.Base(path), len(policy.Topics))

	return nil
}

// validatePolicy checks if a policy is valid
func (l *Loader) validatePolicy(policy *OrgPolicy) error {
	if policy.OrgID == "" {
		return fmt.Errorf("org_id is required")
	}

	if policy.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate each topic policy
	for i, topicPolicy := range policy.Topics {
		if topicPolicy.Pattern == "" {
			return fmt.Errorf("topic[%d]: pattern is required", i)
		}

		// Validate rules
		for j, rule := range topicPolicy.Publish {
			if err := l.validateRule(&rule); err != nil {
				return fmt.Errorf("topic[%d].publish[%d]: %w", i, j, err)
			}
		}
		for j, rule := range topicPolicy.Subscribe {
			if err := l.validateRule(&rule); err != nil {
				return fmt.Errorf("topic[%d].subscribe[%d]: %w", i, j, err)
			}
		}
	}

	return nil
}

// validateRule checks if a rule is valid
func (l *Loader) validateRule(rule *Rule) error {
	if rule.IdentityPattern == "" {
		return fmt.Errorf("identity pattern is required")
	}

	if rule.Type != "" && rule.Type != "api_key" && rule.Type != "user" {
		return fmt.Errorf("invalid type %q (must be 'api_key' or 'user')", rule.Type)
	}

	return nil
}

// watch monitors the policy directory for changes
func (l *Loader) watch() {
	debounce := time.NewTimer(0)
	<-debounce.C // Drain initial timer

	for {
		select {
		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}

			// Only react to write/create/remove events on YAML files
			if !isYAMLFile(event.Name) {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				// Debounce: wait for 100ms of inactivity before reloading
				debounce.Reset(100 * time.Millisecond)
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("ERROR: Policy watcher error: %v\n", err)

		case <-debounce.C:
			fmt.Println("INFO: Policy files changed, reloading...")
			if err := l.loadAll(); err != nil {
				fmt.Printf("ERROR: Failed to reload policies: %v\n", err)
			}

		case <-l.stopChan:
			return
		}
	}
}

// GetPolicy returns the policy for an organization
func (l *Loader) GetPolicy(orgID string) *OrgPolicy {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.policies[orgID]
}

// Close stops the loader and releases resources
func (l *Loader) Close() error {
	close(l.stopChan)
	return l.watcher.Close()
}

func isYAMLFile(filename string) bool {
	ext := filepath.Ext(filename)
	return ext == ".yaml" || ext == ".yml"
}
