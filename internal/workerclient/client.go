package workerclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Client struct {
	Command string
	Script  string
}

type Session struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	request uint64
	mu      sync.Mutex
}

type HealthResponse struct {
	Status    string     `json:"status"`
	Providers []Provider `json:"providers"`
}

type Provider struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	DefaultModel string `json:"default_model"`
}

type UnderstandMediaRequest struct {
	FileID     int64                  `json:"file_id"`
	MediaType  string                 `json:"media_type"`
	FilePath   string                 `json:"file_path"`
	FramePaths []string               `json:"frame_paths,omitempty"`
	Context    UnderstandMediaContext `json:"context"`
}

type UnderstandMediaContext struct {
	AllowSensitiveLabels bool   `json:"allow_sensitive_labels"`
	MaxTags              int    `json:"max_tags"`
	Language             string `json:"language"`
}

type UnderstandMediaResult struct {
	RawTags              []string              `json:"raw_tags"`
	CanonicalCandidates  []UnderstandTagResult `json:"canonical_candidates"`
	Summary              string                `json:"summary"`
	SensitiveTags        []string              `json:"sensitive_tags"`
	QualityHints         []string              `json:"quality_hints"`
	StructuredAttributes map[string]any        `json:"structured_attributes"`
	Confidence           float64               `json:"confidence"`
	Provider             string                `json:"provider"`
	Model                string                `json:"model"`
	RawResponse          map[string]any        `json:"raw_response"`
}

type UnderstandTagResult struct {
	Namespace  string  `json:"namespace"`
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

type EmbedMediaRequest struct {
	EmbeddingType string            `json:"embedding_type,omitempty"`
	MediaType  string            `json:"media_type"`
	FilePath   string            `json:"file_path,omitempty"`
	ImagePHash string            `json:"image_phash,omitempty"`
	Frames     []EmbedFrameInput `json:"frames,omitempty"`
}

type EmbedFrameInput struct {
	FrameID   int64  `json:"frame_id"`
	FramePath string `json:"frame_path,omitempty"`
	PHash     string `json:"phash,omitempty"`
}

type EmbedMediaResult struct {
	Vector       string             `json:"vector"`
	FrameVectors []EmbedFrameVector `json:"frame_vectors"`
	Provider     string             `json:"provider"`
	Model        string             `json:"model"`
	RawResponse  map[string]any     `json:"raw_response"`
}

type EmbedFrameVector struct {
	FrameID int64  `json:"frame_id"`
	Vector  string `json:"vector"`
}

func (c Client) Start(ctx context.Context) (*Session, error) {
	cmd := exec.CommandContext(ctx, c.Command, c.Script)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if _, err := cmd.StderrPipe(); err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &Session{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

func (s *Session) Close() error {
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
	return nil
}

func (s *Session) HealthCheck(ctx context.Context) (HealthResponse, error) {
	var payload HealthResponse
	if err := s.requestInto(ctx, "health_check", nil, &payload); err != nil {
		return HealthResponse{}, err
	}
	return payload, nil
}

func (s *Session) ListModels(ctx context.Context) ([]Provider, error) {
	var payload struct {
		Providers []Provider `json:"providers"`
	}
	if err := s.requestInto(ctx, "list_models", nil, &payload); err != nil {
		return nil, err
	}
	return payload.Providers, nil
}

func (s *Session) UnderstandMedia(ctx context.Context, input UnderstandMediaRequest) (UnderstandMediaResult, error) {
	var payload UnderstandMediaResult
	if err := s.requestInto(ctx, "understand_media", input, &payload); err != nil {
		return UnderstandMediaResult{}, err
	}
	return payload, nil
}

func (s *Session) EmbedMedia(ctx context.Context, input EmbedMediaRequest) (EmbedMediaResult, error) {
	var payload EmbedMediaResult
	if err := s.requestInto(ctx, "embed_media", input, &payload); err != nil {
		return EmbedMediaResult{}, err
	}
	return payload, nil
}

func (s *Session) requestInto(ctx context.Context, messageType string, payload any, out any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.request++
	requestID := fmt.Sprintf("req-%d", s.request)

	request := map[string]any{
		"request_id": requestID,
		"type":       messageType,
	}
	if payload != nil {
		request["payload"] = payload
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if _, err := s.stdin.Write(append(encoded, '\n')); err != nil {
		return err
	}

	type resultEnvelope struct {
		RequestID string          `json:"request_id"`
		Type      string          `json:"type"`
		OK        bool            `json:"ok"`
		Payload   json.RawMessage `json:"payload"`
		Error     *struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}

	done := make(chan error, 1)
	go func() {
		line, err := s.stdout.ReadBytes('\n')
		if err != nil {
			done <- err
			return
		}
		var envelope resultEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			done <- err
			return
		}
		if envelope.RequestID != requestID {
			done <- fmt.Errorf("unexpected request id: got %s want %s", envelope.RequestID, requestID)
			return
		}
		if !envelope.OK {
			if envelope.Error != nil {
				done <- fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
				return
			}
			done <- fmt.Errorf("worker returned non-ok response")
			return
		}
		done <- json.Unmarshal(envelope.Payload, out)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
