package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/filipexyz/notif/internal/cli/output"
	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
)

// Commands blocked in shell mode for security
var blockedCommands = map[string]bool{
	"auth":   true, // Don't allow saving credentials to server filesystem
	"config": true, // Don't allow modifying server config
}

// TutorialStep represents a step in the interactive tutorial
type TutorialStep struct {
	Title       string
	Description string
	Code        string   // SDK code example
	Command     string   // CLI command to try
	Hint        string   // Hint shown after command
	Validators  []string // Command prefixes that complete this step
}

var tutorialSteps = []TutorialStep{
	{
		Title:       "Step 1/3: Publish Events",
		Description: "When something happens in your app, emit an event:",
		Code: `import { Notif } from 'notif.sh'

const notif = new Notif()
await notif.emit('orders.created', { id: '123', total: 99 })`,
		Command:    `emit orders.created '{"id": "123", "total": 99}'`,
		Hint:       "Your event will appear in the live stream above",
		Validators: []string{"emit"},
	},
	{
		Title:       "Step 2/3: Subscribe to Events",
		Description: "React to events in real-time:",
		Code: `for await (const event of notif.subscribe('orders.*')) {
  console.log('New order:', event.data)
  // Process the order...
}`,
		Command:    "events list",
		Hint:       "You can filter events by topic with --topic",
		Validators: []string{"events"},
	},
	{
		Title:       "Step 3/3: Webhooks for HTTP Services",
		Description: "Send events to your HTTP endpoints automatically:",
		Code:        "",
		Command:     "webhooks list",
		Hint:        "Create webhooks with: webhooks create --url <url> --topics <pattern>",
		Validators:  []string{"webhooks"},
	},
}

// Shell manages the interactive shell session
type Shell struct {
	out          *output.Output
	client       *client.Client
	subscription *client.Subscription
	eventChan    chan *client.Event
	inputChan    chan string
	quit         chan struct{}
	mu           sync.Mutex

	// Tutorial state
	tutorialStep    int
	tutorialSkipped bool
	showTutorial    bool
}

func newShell() *Shell {
	return &Shell{
		out:          out,
		eventChan:    make(chan *client.Event, 100),
		inputChan:    make(chan string),
		quit:         make(chan struct{}),
		showTutorial: os.Getenv("NOTIF_SKIP_TUTORIAL") == "",
	}
}

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Interactive shell mode",
	Long:  `Start an interactive shell for running notif commands with live event streaming.`,
	Run: func(cmd *cobra.Command, args []string) {
		shell := newShell()
		shell.Run()
	},
}

func (s *Shell) Run() {
	// Create client for subscriptions
	apiKey := ""
	if jwt := os.Getenv("NOTIF_JWT"); jwt != "" {
		apiKey = jwt
	} else if cfg != nil && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}

	if apiKey != "" {
		s.client = client.New(apiKey, client.WithServer(serverURL))
	}

	// Print welcome and start subscription
	s.printWelcome()

	// Start background event subscription
	if s.client != nil {
		go s.subscribeToAll()
	}

	// Start input reader
	go s.readInput()

	// Show first tutorial step
	if s.showTutorial && !s.tutorialSkipped {
		s.printTutorialStep()
	}

	// Main event loop
	for {
		select {
		case event, ok := <-s.eventChan:
			if !ok {
				continue
			}
			s.displayEvent(event)
		case input, ok := <-s.inputChan:
			if !ok {
				return
			}
			if input == "" {
				continue
			}
			shouldExit := s.handleInput(input)
			if shouldExit {
				return
			}
		case <-s.quit:
			return
		}
	}
}

func (s *Shell) printWelcome() {
	fmt.Println()
	fmt.Printf("%s%s notif.sh %s\n", output.Bold, output.Magenta, output.Reset)
	fmt.Println()

	if s.client != nil {
		fmt.Printf("%s→%s Subscribing to all events...%s\n", output.Cyan, output.Reset, output.Reset)
	} else {
		fmt.Printf("%s!%s No auth configured - events won't stream%s\n", output.Yellow, output.Reset, output.Reset)
	}
	fmt.Println()
}

func (s *Shell) subscribeToAll() {
	ctx := context.Background()
	sub, err := s.client.Subscribe(ctx, []string{"*"}, client.SubscribeOptions{
		AutoAck: true,
		From:    "latest",
	})
	if err != nil {
		fmt.Printf("\n%s✗%s Failed to subscribe: %v%s\n", output.Red, output.Reset, err, output.Reset)
		s.printPrompt()
		return
	}

	s.mu.Lock()
	s.subscription = sub
	s.mu.Unlock()

	fmt.Printf("%s✓%s Connected - live events streaming%s\n", output.Green, output.Reset, output.Reset)
	s.printEventHeader()
	fmt.Println()
	s.printPrompt()

	// Read events into channel
	for event := range sub.Events() {
		select {
		case s.eventChan <- event:
		case <-s.quit:
			return
		}
	}
}

func (s *Shell) printEventHeader() {
	fmt.Printf("\n%s┌─ Live Events ─────────────────────────────────────────────────┐%s\n", output.Gray, output.Reset)
}

func (s *Shell) displayEvent(e *client.Event) {
	// Clear current line and move up (to overwrite prompt)
	fmt.Print("\033[2K\r")

	// Format event
	timestamp := e.Timestamp.Format("15:04:05")
	topic := s.colorTopic(e.Topic)
	data := truncateJSON(e.Data, 50)

	fmt.Printf("%s│%s %s%s%s %s %s%s%s\n",
		output.Gray, output.Reset,
		output.Gray, timestamp, output.Reset,
		topic,
		output.Gray, data, output.Reset,
	)

	// Reprint prompt
	s.printPrompt()
}

func (s *Shell) colorTopic(topic string) string {
	var color string
	switch {
	case strings.HasPrefix(topic, "orders"):
		color = output.Green
	case strings.HasPrefix(topic, "users"):
		color = output.Blue
	case strings.HasPrefix(topic, "payments"):
		color = output.Magenta
	case strings.HasPrefix(topic, "test"):
		color = output.Yellow
	default:
		color = output.Cyan
	}
	return color + topic + output.Reset
}

func truncateJSON(data json.RawMessage, maxLen int) string {
	s := string(data)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func (s *Shell) readInput() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		if !scanner.Scan() {
			close(s.inputChan)
			return
		}
		line := strings.TrimSpace(scanner.Text())
		s.inputChan <- line
	}
}

func (s *Shell) handleInput(input string) bool {
	// Check for exit
	if input == "exit" || input == "quit" {
		s.cleanup()
		return true
	}

	// Check for tutorial commands
	if input == "skip" && s.showTutorial && !s.tutorialSkipped {
		s.tutorialSkipped = true
		fmt.Printf("\n%s→%s Tutorial skipped. Type 'help' for available commands.%s\n\n", output.Cyan, output.Reset, output.Reset)
		s.printPrompt()
		return false
	}

	if input == "tutorial" {
		s.tutorialStep = 0
		s.tutorialSkipped = false
		s.showTutorial = true
		s.printTutorialStep()
		s.printPrompt()
		return false
	}

	// Parse and execute command
	cmdArgs := parseArgs(input)
	if len(cmdArgs) == 0 {
		s.printPrompt()
		return false
	}

	// Block dangerous commands
	if blockedCommands[cmdArgs[0]] {
		s.out.Error("command '%s' is not available in shell mode", cmdArgs[0])
		fmt.Println()
		s.printPrompt()
		return false
	}

	// Execute command
	rootCmd.SetArgs(cmdArgs)
	rootCmd.Execute()
	fmt.Println()

	// Check if command completes current tutorial step
	if s.showTutorial && !s.tutorialSkipped && s.tutorialStep < len(tutorialSteps) {
		step := tutorialSteps[s.tutorialStep]
		for _, prefix := range step.Validators {
			if strings.HasPrefix(cmdArgs[0], prefix) {
				// Show hint
				if step.Hint != "" {
					fmt.Printf("%s↑ %s%s\n\n", output.Gray, step.Hint, output.Reset)
				}
				// Advance to next step
				s.tutorialStep++
				if s.tutorialStep < len(tutorialSteps) {
					s.printTutorialStep()
				} else {
					s.printTutorialComplete()
				}
				break
			}
		}
	}

	s.printPrompt()
	return false
}

func (s *Shell) printTutorialStep() {
	if s.tutorialStep >= len(tutorialSteps) {
		return
	}

	step := tutorialSteps[s.tutorialStep]

	fmt.Println()
	fmt.Printf("%s┌─────────────────────────────────────────────────────────────────┐%s\n", output.Gray, output.Reset)
	fmt.Printf("%s│%s %s%s%s%s\n", output.Gray, output.Reset, output.Bold, output.Cyan, step.Title, output.Reset)
	fmt.Printf("%s├─────────────────────────────────────────────────────────────────┤%s\n", output.Gray, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s  %s%s\n", output.Gray, output.Reset, step.Description, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)

	// Show code example if present
	if step.Code != "" {
		fmt.Printf("%s│%s  %s┌─ TypeScript ───────────────────────────────────────────────┐%s\n", output.Gray, output.Reset, output.Gray, output.Reset)
		for _, line := range strings.Split(step.Code, "\n") {
			fmt.Printf("%s│%s  %s│%s %s%s%s\n", output.Gray, output.Reset, output.Gray, output.Reset, output.Green, line, output.Reset)
		}
		fmt.Printf("%s│%s  %s└────────────────────────────────────────────────────────────┘%s\n", output.Gray, output.Reset, output.Gray, output.Reset)
		fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	}

	fmt.Printf("%s│%s  Try it now:%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s  %snotif>%s %s%s%s\n", output.Gray, output.Reset, output.Magenta, output.Reset, output.Yellow, step.Command, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s  %s(type 'skip' to skip tutorial)%s\n", output.Gray, output.Reset, output.Gray, output.Reset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────────┘%s\n", output.Gray, output.Reset)
	fmt.Println()
}

func (s *Shell) printTutorialComplete() {
	fmt.Println()
	fmt.Printf("%s┌─────────────────────────────────────────────────────────────────┐%s\n", output.Gray, output.Reset)
	fmt.Printf("%s│%s %s%sYou're ready to build!%s%s\n", output.Gray, output.Reset, output.Bold, output.Green, output.Reset, output.Reset)
	fmt.Printf("%s├─────────────────────────────────────────────────────────────────┤%s\n", output.Gray, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s  Install the SDK:%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s    %snpm install notif.sh%s       # TypeScript%s\n", output.Gray, output.Reset, output.Cyan, output.Gray, output.Reset)
	fmt.Printf("%s│%s    %spip install notifsh%s        # Python%s\n", output.Gray, output.Reset, output.Cyan, output.Gray, output.Reset)
	fmt.Printf("%s│%s    %sgo get github.com/filipexyz/notif/pkg/client%s  # Go%s\n", output.Gray, output.Reset, output.Cyan, output.Gray, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s  Get your API key:%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s    %snotif>%s api-keys create --name \"my-app\"%s\n", output.Gray, output.Reset, output.Magenta, output.Yellow, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s│%s  Docs: %shttps://notif.sh/docs%s%s\n", output.Gray, output.Reset, output.Blue, output.Reset, output.Reset)
	fmt.Printf("%s│%s%s\n", output.Gray, output.Reset, output.Reset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────────┘%s\n", output.Gray, output.Reset)
	fmt.Println()
}

func (s *Shell) printPrompt() {
	fmt.Printf("%snotif>%s ", output.Magenta, output.Reset)
}

func (s *Shell) cleanup() {
	close(s.quit)
	s.mu.Lock()
	if s.subscription != nil {
		s.subscription.Close()
	}
	s.mu.Unlock()
}

// parseArgs splits a command line into arguments, respecting quotes
func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range line {
		switch {
		case (r == '"' || r == '\'') && !inQuote:
			inQuote = true
			quoteChar = r
		case r == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
