package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

// store provides persistent disk storage for Todaily app data using Bolt, a key/value database.
//
// To visualize the bucket/key heirarchy:
// |---
// | meta
// |   habits -> []habit
// |---
// | dailyRecords
// |   [YYMMDD] -> []habit
// |---
// | dailySummaries
// |   [YYMMDD] -> dailySummary
// |---
type store struct {
	db *bbolt.DB
}

func openStore(fpath string) (*store, error) {
	if fpath == "" {
		defDir, err := defaultDataDir()
		if err != nil {
			return nil, err
		}
		fpath = filepath.Join(defDir, "db.todaily")
	}
	// Ensure the parent directory exists first and then open the db.
	if err := os.MkdirAll(path.Dir(fpath), os.ModePerm); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(fpath, 0o644, nil)
	if err != nil {
		return nil, fmt.Errorf("opening bolt db: %w", err)
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("meta")) != nil {
			return nil
		}
		for _, bucketName := range []string{"meta", "dailyRecords", "dailySummaries"} {
			_, err := tx.CreateBucket([]byte(bucketName))
			if err != nil {
				return fmt.Errorf("creating bucket %q: %w", bucketName, err)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("initializing bolt db: %w", err)
	}
	return &store{db: db}, nil
}

func (s *store) getSummaries() (map[string]dailySummary, error) {
	sums := make(map[string]dailySummary)
	return sums, s.db.View(func(tx *bbolt.Tx) error {
		tx.Bucket([]byte("dailySummaries")).ForEach(func(k, v []byte) error {
			var summary dailySummary
			if err := json.Unmarshal(v, &summary); err != nil {
				return fmt.Errorf("decoding summary for %q: %w", string(k), err)
			}
			sums[string(k)] = summary
			return nil
		})
		return nil
	})
}

func (s *store) getHabits() (items []habit, _ error) {
	return items, s.db.View(func(tx *bbolt.Tx) error {
		meta := tx.Bucket([]byte("meta"))
		if err := get(meta, []byte("habits"), &items); err != nil {
			return fmt.Errorf("getting habit template list from meta: %w", err)
		}
		return nil
	})
}

func (s *store) putHabits(items []habit) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		meta := tx.Bucket([]byte("meta"))
		if err := put(meta, []byte("habits"), items); err != nil {
			return fmt.Errorf("putting habit template list into meta: %w", err)
		}
		return nil
	})
}

func (s *store) getHabitsForDay(fmtDate string) (items []habit, _ error) {
	now := time.Now()
	t, err := time.ParseInLocation("060102", fmtDate, now.Location())
	if err != nil {
		return nil, fmt.Errorf("parsing fmtDate %q: %w", fmtDate, err)
	}
	if t.After(now) {
		return nil, fmt.Errorf("requesting habits for %q, a future date", fmtDate)
	}
	return items, s.db.Update(func(tx *bbolt.Tx) (err error) {
		k := []byte(fmtDate)
		dailys := tx.Bucket([]byte("dailyRecords"))
		if dailys.Get(k) == nil {
			meta := tx.Bucket([]byte("meta"))
			var templateList []habit
			if err := get(meta, []byte("habits"), &templateList); err != nil {
				return fmt.Errorf("reading habit template list: %w", err)
			}
			for _, h := range templateList {
				if h.CreatedAt.Before(t) && (h.DeletedAt.IsZero() || h.DeletedAt.After(t)) {
					h.CreatedAt = now
					items = append(items, h)
				}
			}
			if err := put(dailys, k, items); err != nil {
				return fmt.Errorf("inserting habit data for new day %q: %w", fmtDate, err)
			}
			return nil
		}
		if err := get(dailys, k, &items); err != nil {
			return fmt.Errorf("getting habits for %q: %w", fmtDate, err)
		}
		return nil
	})
}

func (s *store) putHabitsForDay(fmtDate string, items []habit) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		k := []byte(fmtDate)
		dailys := tx.Bucket([]byte("dailyRecords"))
		if err := put(dailys, k, items); err != nil {
			return err
		}
		sums := tx.Bucket([]byte("dailySummaries"))
		summary := newSummaryOfList(items)
		if err := put(sums, k, summary); err != nil {
			return fmt.Errorf("setting completion status for %q: %w", fmtDate, err)
		}
		return nil
	})
}

func (s *store) Close() error {
	return s.db.Close()
}

func put(b *bbolt.Bucket, key []byte, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding to json: %w", err)
	}
	if err := b.Put(key, data); err != nil {
		return fmt.Errorf("writing json blob: %w", err)
	}
	return nil
}

func get(b *bbolt.Bucket, key []byte, v any) error {
	data := b.Get(key)
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	return nil
}

// defaultDataDir returns the default location for storing app data
// which is a directory named `.todaily` in the user's home directory.
func defaultDataDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obtaining user home directory: %w", err)
	}
	return filepath.Join(dir, ".todaily"), nil
}
