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
package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ossf/allstar/pkg/enforce"
	"github.com/ossf/allstar/pkg/ghclients"
)

func main() {
	ghc := ghclients.NewGHClients(http.DefaultTransport)

	// Just run for 10 min now
	ctx, cf := context.WithTimeout(context.Background(), (10 * time.Minute))

	var wg sync.WaitGroup
	// Kickoff webhook listener, delayed enforce, reconcile job...
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Print(enforce.EnforceJob(ctx, ghc, (1 * time.Minute)))
	}()
	wg.Wait()
	cf()
}
