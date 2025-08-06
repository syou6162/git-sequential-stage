package executor

import (
	"fmt"
	"io"
)

// MockCommandExecutor is a mock implementation of CommandExecutor for testing
type MockCommandExecutor struct {
	// Commands stores the expected commands and their responses
	Commands map[string]MockResponse
	// ExecutedCommands tracks what commands were actually executed
	ExecutedCommands []ExecutedCommand
}

// MockResponse represents a mocked command response
type MockResponse struct {
	Output []byte
	Error  error
}

// ExecutedCommand represents a command that was executed
type ExecutedCommand struct {
	Name  string
	Args  []string
	Stdin []byte
	Dir   string
}

// NewMockCommandExecutor creates a new mock executor
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		Commands:         make(map[string]MockResponse),
		ExecutedCommands: []ExecutedCommand{},
	}
}

// Execute implements CommandExecutor.Execute
func (m *MockCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
	key := fmt.Sprintf("%s %v", name, args)
	m.ExecutedCommands = append(m.ExecutedCommands, ExecutedCommand{
		Name: name,
		Args: args,
	})

	if response, ok := m.Commands[key]; ok {
		return response.Output, response.Error
	}

	return nil, fmt.Errorf("unexpected command: %s", key)
}

// ExecuteWithStdin implements CommandExecutor.ExecuteWithStdin
func (m *MockCommandExecutor) ExecuteWithStdin(name string, stdin io.Reader, args ...string) ([]byte, error) {
	var stdinData []byte
	if stdin != nil {
		stdinData, _ = io.ReadAll(stdin)
	}

	key := fmt.Sprintf("%s %v", name, args)
	m.ExecutedCommands = append(m.ExecutedCommands, ExecutedCommand{
		Name:  name,
		Args:  args,
		Stdin: stdinData,
	})

	if response, ok := m.Commands[key]; ok {
		return response.Output, response.Error
	}

	return nil, fmt.Errorf("unexpected command: %s", key)
}

// ExecuteInDir implements CommandExecutor.ExecuteInDir
func (m *MockCommandExecutor) ExecuteInDir(dir string, name string, args ...string) ([]byte, error) {
	key := fmt.Sprintf("%s %v", name, args)
	m.ExecutedCommands = append(m.ExecutedCommands, ExecutedCommand{
		Name: name,
		Args: args,
		Dir:  dir,
	})

	if response, ok := m.Commands[key]; ok {
		return response.Output, response.Error
	}

	return nil, fmt.Errorf("unexpected command in dir %s: %s", dir, key)
}
