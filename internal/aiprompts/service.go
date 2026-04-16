package aiprompts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Settings struct {
	UnderstandingExtraPrompt string `json:"understanding_extra_prompt"`
}

type Service struct {
	Path string
	mu   sync.Mutex
}

func (s *Service) GetSettings(_ context.Context) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *Service) UpdateSettings(_ context.Context, input Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input.UnderstandingExtraPrompt = strings.TrimSpace(input.UnderstandingExtraPrompt)
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return Settings{}, err
	}
	body, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return Settings{}, err
	}
	if err := os.WriteFile(s.Path, append(body, '\n'), 0o644); err != nil {
		return Settings{}, err
	}
	return input, nil
}

func (s *Service) read() (Settings, error) {
	if strings.TrimSpace(s.Path) == "" {
		return Settings{}, nil
	}
	body, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	if len(body) == 0 {
		return Settings{}, nil
	}
	var result Settings
	if err := json.Unmarshal(body, &result); err != nil {
		return Settings{}, err
	}
	result.UnderstandingExtraPrompt = strings.TrimSpace(result.UnderstandingExtraPrompt)
	return result, nil
}
