package local

import (
	"encoding/json"
	"errors"
	"os"
)

type ConfigStore struct {
	Files *FileStore
}

func (s *ConfigStore) Read(name string, fallback map[string]any) (map[string]any, error) {
	if name == "" {
		name = "settings.json"
	}
	var out map[string]any
	err := s.Files.ReadJSON(name, &out)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && fallback != nil {
			return fallback, nil
		}
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func (s *ConfigStore) Write(name string, value any) (string, error) {
	if name == "" {
		name = "settings.json"
	}
	return s.Files.WriteJSON(name, value)
}

func (s *ConfigStore) Get(name, key string, fallback any) (any, error) {
	config, err := s.Read(name, map[string]any{})
	if err != nil {
		return nil, err
	}
	value, ok := config[key]
	if !ok {
		return fallback, nil
	}
	return value, nil
}

func (s *ConfigStore) Set(name, key string, value any) (string, error) {
	config, err := s.Read(name, map[string]any{})
	if err != nil {
		return "", err
	}
	config[key] = value
	return s.Write(name, config)
}

func (s *ConfigStore) Delete(name, key string) (string, error) {
	config, err := s.Read(name, map[string]any{})
	if err != nil {
		return "", err
	}
	delete(config, key)
	return s.Write(name, config)
}

func (s *ConfigStore) ReadInto(name string, out any) error {
	raw, err := s.Files.ReadBytes(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
