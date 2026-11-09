// Copyright 2021 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ghclients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-cmp/cmp"

	"github.com/ossf/allstar/pkg/config/operator"
)

//nolint:paralleltest // Replaces transport factories and operator configuration globals.
func TestGet(t *testing.T) {
	setupGHClientTest(t)
	called := 0
	ghinstallationNewAppsTransport = func(http.RoundTripper, int64,
		[]byte,
	) (*ghinstallation.AppsTransport, error) {
		called++
		return &ghinstallation.AppsTransport{BaseURL: fmt.Sprint(0)}, nil
	}
	ghinstallationNew = func(r http.RoundTripper, a int64, i int64,
		f []byte,
	) (*ghinstallation.Transport, error) {
		called++
		return &ghinstallation.Transport{BaseURL: fmt.Sprint(i)}, nil
	}
	getKeyFromSecret = func(ctx context.Context, keySecretVal string) ([]byte, error) {
		return nil, nil
	}
	ghc, err := NewGHClients(context.Background(), http.DefaultTransport)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	c1, err := ghc.Get(0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	called = 0
	c2, err := ghc.Get(0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if called != 0 {
		t.Errorf("Did not used cached client")
	}
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Got wrong client")
	}

	i1, err := ghc.Get(123)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	called = 0
	i2, err := ghc.Get(123)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if called != 0 {
		t.Errorf("Did not used cached client")
	}
	if !reflect.DeepEqual(i1, i2) {
		t.Errorf("Got wrong client")
	}
}

//nolint:paralleltest // Replaces transport factories and operator configuration globals.
func TestGetKey(t *testing.T) {
	setupGHClientTest(t)

	tests := []struct {
		Name       string
		KeySecret  string
		PrivateKey string
		ExpKey     string
	}{
		{
			Name:       "HasOnlyPrivateKey",
			KeySecret:  "direct",
			PrivateKey: "foo",
			ExpKey:     "foo",
		},
		{
			Name:       "HasOnlyKeySecret",
			KeySecret:  "bar",
			PrivateKey: "",
			ExpKey:     "bar",
		},
		{
			Name:       "HasPrivateKeyAndSecret",
			KeySecret:  "foo",
			PrivateKey: "bar",
			ExpKey:     "foo",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			privateKey = test.PrivateKey
			keySecret = test.KeySecret
			getKeyFromSecret = func(ctx context.Context, keySecretVal string) ([]byte, error) {
				return []byte(keySecretVal), nil
			}

			ghc, err := NewGHClients(context.Background(), http.DefaultTransport)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff([]byte(test.ExpKey), ghc.key); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

//nolint:paralleltest // Replaces transport factories and operator configuration globals.
func TestGitHubEnterpriseClient(t *testing.T) {
	setupGHClientTest(t)
	tests := []struct {
		Name                string
		GitHubEnterpriseUrl string
		installId           int64
		expectedBaseUrl     string
		expectedUploadUrl   string
	}{
		{
			Name:                "ZeroAppId",
			GitHubEnterpriseUrl: "https://ghezero.example.com",
			installId:           0,
			expectedBaseUrl:     "https://ghezero.example.com/api/v3/",
			expectedUploadUrl:   "https://ghezero.example.com/api/uploads/",
		},
		{
			Name:                "NonZeroAppId",
			GitHubEnterpriseUrl: "https://ghenonzero.example.com",
			installId:           123,
			expectedBaseUrl:     "https://ghenonzero.example.com/api/v3/",
			expectedUploadUrl:   "https://ghenonzero.example.com/api/uploads/",
		},
		{
			Name:                "NoGitHubEnterpriseUrl",
			GitHubEnterpriseUrl: "",
			installId:           0,
			expectedBaseUrl:     "https://api.github.com/",
			expectedUploadUrl:   "https://uploads.github.com/",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			operator.GitHubEnterpriseUrl = test.GitHubEnterpriseUrl

			ghc, err := NewGHClients(context.Background(), http.DefaultTransport)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			c, err := ghc.Get(test.installId)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if c.BaseURL.String() != test.expectedBaseUrl {
				t.Errorf("Did not read GitHub instance URL from operator config.\nExpected %s, got %s", test.expectedBaseUrl, c.BaseURL.String())
			}
			if c.UploadURL.String() != test.expectedUploadUrl {
				t.Errorf("Did not read GitHub upload URL from operator config\nExpected %s, got %s", test.expectedUploadUrl, c.UploadURL.String())
			}
		})
	}
}

func setupGHClientTest(t *testing.T) {
	t.Helper()
	oldAppsTransport, oldTransport := ghinstallationNewAppsTransport, ghinstallationNew
	oldGetSecret := getKeyFromSecret
	oldPrivateKey, oldKeySecret := privateKey, keySecret
	oldEnterpriseURL := operator.GitHubEnterpriseUrl
	t.Cleanup(func() {
		ghinstallationNewAppsTransport, ghinstallationNew = oldAppsTransport, oldTransport
		getKeyFromSecret = oldGetSecret
		privateKey, keySecret = oldPrivateKey, oldKeySecret
		operator.GitHubEnterpriseUrl = oldEnterpriseURL
	})
	ghinstallationNewAppsTransport = func(http.RoundTripper, int64, []byte) (*ghinstallation.AppsTransport, error) {
		return &ghinstallation.AppsTransport{BaseURL: fmt.Sprint(0)}, nil
	}
	ghinstallationNew = func(_ http.RoundTripper, _ int64, i int64, _ []byte) (*ghinstallation.Transport, error) {
		return &ghinstallation.Transport{BaseURL: fmt.Sprint(i)}, nil
	}
	getKeyFromSecret = func(_ context.Context, secret string) ([]byte, error) { return []byte(secret), nil }
}

func TestGetKeyFromSecretReal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		decoder string
		wantErr error
	}{
		{name: "bytes", decoder: "bytes"},
		{name: "string rejected", decoder: "string", wantErr: errUnexpectedSecretType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			secret := "test-private-key"
			secretPath := filepath.Join(t.TempDir(), "key")
			if err := os.WriteFile(secretPath, []byte(secret), 0o600); err != nil {
				t.Fatal(err)
			}
			secretURL := &url.URL{
				Scheme:   "file",
				Path:     secretPath,
				RawQuery: url.Values{"decoder": {tt.decoder}}.Encode(),
			}
			key, err := getKeyFromSecretReal(t.Context(), secretURL.String())
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Error = %v, want %v", err, tt.wantErr)
				}
				if key != nil {
					t.Error("Expected no key for an invalid secret type")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff([]byte(secret), key); diff != "" {
				t.Errorf("Unexpected key. (-want +got):\n%s", diff)
			}
		})
	}
}
