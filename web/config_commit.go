package web

import (
	"errors"
	"fmt"

	"github.com/unimap/project/internal/config"
)

var (
	errInvalidConfig = errors.New("invalid configuration")
	errPersistConfig = errors.New("persist configuration")
)

// currentConfig returns an isolated snapshot of the latest committed config.
// s.config is the immutable startup snapshot and is only a fallback for tests
// that intentionally construct a Server without a config Manager.
func (s *Server) currentConfig() *config.Config {
	if s.configManager != nil {
		if cfg := s.configManager.GetConfig(); cfg != nil {
			return cfg
		}
	}
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	if s.config == nil {
		return nil
	}
	return s.config.Clone()
}

func (s *Server) allowedOrigins() []string {
	return allowedOriginsFromConfig(s.currentConfig())
}

// updateConfig serializes a copy-on-write configuration transaction. The
// candidate is persisted and published by Manager before it is returned.
// Runtime side effects must be applied by the caller after this method returns.
func (s *Server) updateConfig(mutate func(*config.Config) error) (*config.Config, error) {
	if s.configManager == nil {
		s.configMutex.Lock()
		defer s.configMutex.Unlock()
		if s.config == nil {
			return nil, fmt.Errorf("config is not loaded")
		}
		candidate := s.config.Clone()
		if err := mutate(candidate); err != nil {
			return nil, err
		}
		validator := config.NewManager("")
		validator.ApplyDefaults(candidate)
		if err := validator.Validate(candidate); err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidConfig, err)
		}
		s.config = candidate // test/embedded mode: no persistence adapter was supplied
		return candidate, nil
	}
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	candidate := s.configManager.GetConfig()
	if candidate == nil && s.config != nil {
		candidate = s.config.Clone()
	}
	if candidate == nil {
		return nil, fmt.Errorf("config is not loaded")
	}
	if err := mutate(candidate); err != nil {
		return nil, err
	}
	s.configManager.ApplyDefaults(candidate)
	if err := s.configManager.Validate(candidate); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidConfig, err)
	}
	if err := s.configManager.SaveConfig(candidate); err != nil {
		return nil, fmt.Errorf("%w: %v", errPersistConfig, err)
	}
	return candidate, nil
}
