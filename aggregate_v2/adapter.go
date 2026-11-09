// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

// Engine is an alias to Sampler, designed for Dataway migration and feature-flag switching.
type Engine = Sampler

// UpdateConfigFromTOML parses TOML bytes into TailSamplingConfigs and updates the sampler.
func (s *DefaultTimeWheelSampler) UpdateConfigFromTOML(token string, tomlData []byte) error {
	if s.closed.Load() {
		return ErrSamplerClosed
	}
	if len(tomlData) == 0 {
		return s.UpdateConfig(token, nil)
	}

	var cfg TailSamplingConfigs
	if err := toml.Unmarshal(tomlData, &cfg); err != nil {
		return fmt.Errorf("unmarshal tail sampling toml failed: %w", err)
	}

	return s.UpdateConfig(token, &cfg)
}

// UpdateConfigFromJSON parses JSON bytes into TailSamplingConfigs and updates the sampler.
func (s *DefaultTimeWheelSampler) UpdateConfigFromJSON(token string, jsonData []byte) error {
	if s.closed.Load() {
		return ErrSamplerClosed
	}
	if len(jsonData) == 0 {
		return s.UpdateConfig(token, nil)
	}

	var cfg TailSamplingConfigs
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return fmt.Errorf("unmarshal tail sampling json failed: %w", err)
	}

	return s.UpdateConfig(token, &cfg)
}

// Ensure DefaultTimeWheelSampler implements Engine at compile-time.
var _ Engine = (*DefaultTimeWheelSampler)(nil)

// NewEngine is a convenience factory that returns an Engine.
func NewEngine(cfg *SamplerConfig) (Engine, error) {
	return NewSampler(cfg)
}

// RunTimeWheelLoop starts a background ticking loop for the sampler, executing AdvanceTime every second
// and dispatching kept packets to the sink callback. It terminates when canceled or sampler is closed.
func RunTimeWheelLoop(engine Engine, tickInterval time.Duration, sink func(kept []*DataPacket, dropped int)) (stop func()) {
	if tickInterval <= 0 {
		tickInterval = time.Second
	}
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				kept, dropped, err := engine.AdvanceTime(now)
				if err != nil {
					continue
				}
				if sink != nil && (len(kept) > 0 || dropped > 0) {
					sink(kept, dropped)
				}
			}
		}
	}()

	return func() {
		close(done)
	}
}
