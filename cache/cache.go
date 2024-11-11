package cache

import (
	"encoding/json"
	"os"
	"time"
)

type CacheEntry struct {
	Version     string    `json:"version"`
	Items       []string  `json:"items"`
	LastUpdated time.Time `json:"lastUpdated"`
}

type Cache struct {
	Entries []CacheEntry `json:"entries"`
}

func LoadCache(path string) (*Cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{}, nil
		}
		return nil, err
	}

	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func (c *Cache) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
