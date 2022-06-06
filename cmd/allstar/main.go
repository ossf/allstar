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
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ossf/allstar/pkg/enforce"
	"github.com/ossf/allstar/pkg/ghclients"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	setupLog()
	ctx, cf := context.WithCancel(context.Background())

	ghc, err := ghclients.NewGHClients(ctx, http.DefaultTransport)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Could not load app secret, shutting down")
	}

	boolArgPtr := flag.Bool("once", false, "Run EnforceAll once, instead of in a contiuous loop.")
	flag.Parse()

	if *boolArgPtr {
		_, err := enforce.EnforceAll(ctx, ghc)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("Unexpected error enforcing policies.")
		}
	} else {
		var wg sync.WaitGroup
		// Kickoff webhook listener, delayed enforce, reconcile job...
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info().
				Err(enforce.EnforceJob(ctx, ghc, (5 * time.Minute))).
				Msg("Enforce job shutting down.")
		}()
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		s := <-sigs
		cf()
		log.Info().
			Str("signal", s.String()).
			Msg("Signal received, shutting down gracefully")
		wg.Wait()
	}
}

func setupLog() {
	// Match expected values in GCP
	zerolog.LevelFieldName = "severity"
	zerolog.LevelTraceValue = "DEFAULT"
	zerolog.LevelDebugValue = "DEBUG"
	zerolog.LevelInfoValue = "INFO"
	zerolog.LevelWarnValue = "WARNING"
	zerolog.LevelErrorValue = "ERROR"
	zerolog.LevelFatalValue = "CRITICAL"
	zerolog.LevelPanicValue = "CRITICAL"
}
