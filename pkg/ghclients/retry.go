// Copyright 2026 Allstar Authors

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
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

// newRetryRoundTripper retries the transient gateway errors that GitHub uses
// when an idempotent API request times out. The retryable client applies
// exponential backoff and stops when the request context is canceled.
func newRetryRoundTripper(base http.RoundTripper) http.RoundTripper {
	client := retryablehttp.NewClient()
	client.HTTPClient.Transport = base
	client.Logger = nil
	client.CheckRetry = retryGatewayErrors
	client.ErrorHandler = retryablehttp.PassthroughErrorHandler

	return &retryablehttp.RoundTripper{Client: client}
}

func retryGatewayErrors(ctx context.Context, resp *http.Response, _ error) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	if resp.Request == nil || !isIdempotentMethod(resp.Request.Method) {
		return false, nil
	}

	return resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusGatewayTimeout, nil
}

func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace,
		http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}
