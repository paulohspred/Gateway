package rapid

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
)

const maxRapidWebResponseBytes = 2 << 20

var ErrRapidSemanticBindingRequired = errors.New("rapid semantic binding required")

type WebReaderOptions struct {
	BaseURL   string
	Username  string
	Password  string
	Timeout   time.Duration
	Transport http.RoundTripper
	Now       func() time.Time
}

type WebReader struct {
	baseURL  *url.URL
	username string
	password string
	client   *http.Client
	now      func() time.Time

	mu            sync.Mutex
	authenticated bool
}

func NewWebReader(options WebReaderOptions) (*WebReader, error) {
	baseURL, err := validateRapidWebURL(options.BaseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Username) == "" {
		return nil, errors.New("rapid Web API username is required")
	}
	if options.Password == "" {
		return nil, errors.New("rapid Web API password is required")
	}

	timeout := options.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if timeout < time.Second || timeout > time.Minute {
		return nil, errors.New("rapid Web API timeout must be between 1s and 1m")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create rapid Web API cookie jar: %w", err)
	}
	transport := options.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &WebReader{
		baseURL:  baseURL,
		username: options.Username,
		password: options.Password,
		client: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   timeout,
		},
		now: now,
	}, nil
}

func (r *WebReader) ReadCurrent(ctx context.Context, channels []int) ([]ChannelData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	channelNumbers, err := normalizeChannelNumbers(channels)
	if err != nil {
		return nil, err
	}
	if len(channelNumbers) == 0 {
		return []ChannelData{}, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureLoginLocked(ctx); err != nil {
		return nil, err
	}

	data, status, err := r.getCurrentLocked(ctx, channelNumbers)
	if err == nil {
		return data, nil
	}
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		return nil, err
	}

	r.authenticated = false
	if err := r.ensureLoginLocked(ctx); err != nil {
		return nil, err
	}
	data, _, err = r.getCurrentLocked(ctx, channelNumbers)
	return data, err
}

func (r *WebReader) ReadAlarms(context.Context, string) ([]monitor.Alarm, error) {
	return nil, ErrRapidSemanticBindingRequired
}

func (r *WebReader) ReadEvents(context.Context, string) ([]monitor.Event, error) {
	return nil, ErrRapidSemanticBindingRequired
}

func (r *WebReader) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authenticated = false
	return r.ensureLoginLocked(ctx)
}

func (r *WebReader) ensureLoginLocked(ctx context.Context) error {
	if r.authenticated {
		return nil
	}

	payload, err := json.Marshal(map[string]string{
		"username": r.username,
		"password": r.password,
	})
	if err != nil {
		return err
	}
	endpoint := r.resolve("Api/Auth/Login")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create rapid Web API login request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("rapid Web API login: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("rapid Web API login returned HTTP %d", response.StatusCode)
	}

	var dto rapidDTO[json.RawMessage]
	if err := decodeRapidJSON(response.Body, &dto); err != nil {
		return fmt.Errorf("decode rapid Web API login response: %w", err)
	}
	if !dto.OK {
		message := strings.TrimSpace(dto.Message)
		if message == "" {
			message = "login rejected"
		}
		return fmt.Errorf("rapid Web API login failed: %s", message)
	}

	r.authenticated = true
	return nil
}

func (r *WebReader) getCurrentLocked(ctx context.Context, channels []int) ([]ChannelData, int, error) {
	endpoint := r.resolve("Api/Main/GetCurData")
	query := endpoint.Query()
	values := make([]string, len(channels))
	for i, channel := range channels {
		values[i] = strconv.Itoa(channel)
	}
	query.Set("cnlNums", strings.Join(values, ","))
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create rapid current-data request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := r.client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("rapid current-data request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("rapid current-data request returned HTTP %d", response.StatusCode)
	}

	var dto rapidDTO[[]rapidCurrentPoint]
	if err := decodeRapidJSON(response.Body, &dto); err != nil {
		return nil, response.StatusCode, fmt.Errorf("decode rapid current-data response: %w", err)
	}
	if !dto.OK {
		message := strings.TrimSpace(dto.Message)
		if message == "" {
			message = "request rejected"
		}
		return nil, response.StatusCode, fmt.Errorf("rapid current-data request failed: %s", message)
	}

	observedAt := r.now().UTC()
	data := make([]ChannelData, 0, len(dto.Data))
	requested := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		requested[channel] = struct{}{}
	}
	seen := make(map[int]struct{}, len(dto.Data))
	for _, point := range dto.Data {
		if _, ok := requested[point.ChannelNumber]; !ok {
			return nil, response.StatusCode, fmt.Errorf("rapid returned unexpected channel %d", point.ChannelNumber)
		}
		if _, ok := seen[point.ChannelNumber]; ok {
			return nil, response.StatusCode, fmt.Errorf("rapid returned duplicate channel %d", point.ChannelNumber)
		}
		seen[point.ChannelNumber] = struct{}{}
		data = append(data, ChannelData{
			ChannelNumber: point.ChannelNumber,
			Value:         point.Value,
			Status:        point.Status,
			ObservedAt:    observedAt,
		})
	}
	return data, response.StatusCode, nil
}

func (r *WebReader) resolve(path string) *url.URL {
	relative := &url.URL{Path: path}
	return r.baseURL.ResolveReference(relative)
}

type rapidDTO[T any] struct {
	OK      bool   `json:"ok"`
	Message string `json:"msg"`
	Data    T      `json:"data"`
}

type rapidCurrentPoint struct {
	ChannelNumber int     `json:"cnlNum"`
	Value         float64 `json:"val"`
	Status        int     `json:"stat"`
}

func decodeRapidJSON(reader io.Reader, target any) error {
	limited := io.LimitReader(reader, maxRapidWebResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > maxRapidWebResponseBytes {
		return errors.New("rapid Web API response exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("rapid Web API returned multiple JSON values")
		}
		return err
	}
	return nil
}

func validateRapidWebURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid rapid Web API URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("rapid Web API URL must use http or https")
	}
	if parsed.User != nil {
		return nil, errors.New("rapid Web API URL must not contain user info")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("rapid Web API URL host is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("rapid Web API URL must not contain query or fragment")
	}
	if !isLoopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("rapid Web API host %q is not loopback", parsed.Hostname())
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizeChannelNumbers(channels []int) ([]int, error) {
	set := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		if channel <= 0 {
			return nil, fmt.Errorf("rapid channel number must be positive, got %d", channel)
		}
		set[channel] = struct{}{}
	}
	out := make([]int, 0, len(set))
	for channel := range set {
		out = append(out, channel)
	}
	sort.Ints(out)
	return out, nil
}

var _ Reader = (*WebReader)(nil)
