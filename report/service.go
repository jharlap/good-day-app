package report

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

// RenderService represents our upstream render service.
type RenderService struct {
	// URL is the render service address.
	URL             string
	CredentialsFile string

	tokenSource oauth2.TokenSource
}

// NewRequest creates a new HTTP request to the Render service.
// If authentication is enabled, an Identity Token is created and added.
func (s *RenderService) NewRequest(method string) (*http.Request, error) {
	req, err := http.NewRequest(method, s.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a TokenSource if none exists.
	if s.tokenSource == nil {
		var opts []idtoken.ClientOption
		if len(s.CredentialsFile) > 0 {
			opts = append(opts, idtoken.WithCredentialsFile(s.CredentialsFile))
		}

		s.tokenSource, err = idtoken.NewTokenSource(ctx, s.URL, opts...)
		if err != nil {
			return nil, fmt.Errorf("idtoken.NewTokenSource: %w", err)
		}
	}

	// Retrieve an identity token. Will reuse tokens until refresh needed.
	token, err := s.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("TokenSource.Token: %w", err)
	}
	token.SetAuthHeader(req)

	return req, nil
}

var renderClient = &http.Client{Timeout: 30 * time.Second}

// Render converts the data to a chart
func (s *RenderService) Render(in []byte) ([]byte, error) {
	req, err := s.NewRequest(http.MethodPost)
	if err != nil {
		return nil, fmt.Errorf("RenderService.NewRequest: %w", err)
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(in))
	defer req.Body.Close()

	resp, err := renderClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Client.Do: %w", err)
	}

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("http.Client.Do: %s (%d): request not OK", http.StatusText(resp.StatusCode), resp.StatusCode)
	}

	return out, nil
}
