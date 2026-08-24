package persistence

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

type Store struct {
	dir          string
	snapshots    string
	eventPath    string
	receipts     string
	receiptCache map[string]*domain.ArchiveIntegrityReceipt
	mu           sync.Mutex
	sequence     int64
}

func Open(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	s := &Store{
		dir:          dir,
		snapshots:    filepath.Join(dir, "snapshots"),
		eventPath:    filepath.Join(dir, "events.jsonl"),
		receipts:     filepath.Join(dir, "receipts"),
		receiptCache: make(map[string]*domain.ArchiveIntegrityReceipt),
	}
	if err := os.MkdirAll(s.snapshots, 0o700); err != nil {
		return nil, fmt.Errorf("创建快照目录: %w", err)
	}
	if err := os.MkdirAll(s.receipts, 0o700); err != nil {
		return nil, fmt.Errorf("创建回执目录: %w", err)
	}
	if err := s.recover(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Create(application *domain.MigrationApplication) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.snapshotPath(application.ID)
	if _, err := os.Stat(path); err == nil {
		return &ConflictError{Expected: 0, Actual: application.Revision}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if duplicate, err := s.findActiveDuplicateLocked(application.TreeCode, application.ID); err != nil {
		return err
	} else if duplicate != nil {
		return &ActiveDuplicateError{ApplicationID: duplicate.ID, Status: duplicate.Status, Revision: duplicate.Revision}
	}
	data := snapshotData{Application: application, Commands: make(map[string]CommandResult)}
	return s.commit(path, data, "create", "")
}

func (s *Store) FindByTreeCode(treeCode string) ([]*domain.MigrationApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findByTreeCodeLocked(domain.NormalizeTreeCode(treeCode))
}

func (s *Store) Load(id string) (*domain.MigrationApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _, err := s.read(s.snapshotPath(id))
	if err != nil {
		return nil, err
	}
	return cloneApplication(data.Application)
}

func (s *Store) List() ([]*domain.MigrationApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.snapshots)
	if err != nil {
		return nil, fmt.Errorf("读取快照目录: %w", err)
	}
	apps := make([]*domain.MigrationApplication, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, _, err := s.read(filepath.Join(s.snapshots, entry.Name()))
		if err != nil {
			return nil, err
		}
		copy, err := cloneApplication(data.Application)
		if err != nil {
			return nil, err
		}
		apps = append(apps, copy)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].UpdatedAt.After(apps[j].UpdatedAt) })
	return apps, nil
}

func (s *Store) Save(expectedRevision int, application *domain.MigrationApplication, operation, requestID string, payload json.RawMessage) (CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.snapshotPath(application.ID)
	data, _, err := s.read(path)
	if err != nil {
		return CommandResult{}, err
	}
	if requestID != "" {
		if previous, ok := data.Commands[commandKey(operation, requestID)]; ok {
			return previous, nil
		}
	}
	if data.Application.Revision != expectedRevision {
		return CommandResult{}, &ConflictError{Expected: expectedRevision, Actual: data.Application.Revision}
	}
	if duplicate, err := s.findActiveDuplicateLocked(application.TreeCode, application.ID); err != nil {
		return CommandResult{}, err
	} else if duplicate != nil {
		return CommandResult{}, &ActiveDuplicateError{ApplicationID: duplicate.ID, Status: duplicate.Status, Revision: duplicate.Revision}
	}
	result := CommandResult{RequestID: requestID, Operation: operation, ApplicationID: application.ID, Revision: application.Revision, Payload: payload, RecordedAt: time.Now().UTC()}
	if requestID != "" {
		data.Commands[commandKey(operation, requestID)] = result
	}
	data.Application = application
	if err := s.commit(path, data, operation, requestID); err != nil {
		return CommandResult{}, err
	}
	return result, nil
}

func (s *Store) SaveReceipt(receipt domain.ArchiveIntegrityReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("编码核验回执: %w", err)
	}
	path := filepath.Join(s.receipts, receipt.ApplicationID+"-"+receipt.ID+".json")
	if err := atomicWrite(path, append(raw, '\n')); err != nil {
		return err
	}
	s.receiptCache[receipt.ApplicationID] = &receipt
	return nil
}

func (s *Store) LatestReceipt(applicationID string) (*domain.ArchiveIntegrityReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.receiptCache[applicationID]; ok {
		return cached, nil
	}
	entries, err := os.ReadDir(s.receipts)
	if err != nil {
		return nil, err
	}
	var latest *domain.ArchiveIntegrityReceipt
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), applicationID+"-") || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.receipts, entry.Name()))
		if err != nil {
			return nil, err
		}
		var receipt domain.ArchiveIntegrityReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return nil, fmt.Errorf("解析核验回执: %w", err)
		}
		if latest == nil || receipt.CheckedAt.After(latest.CheckedAt) {
			copy := receipt
			latest = &copy
		}
	}
	if latest != nil {
		s.receiptCache[applicationID] = latest
	}
	return latest, nil
}

func (s *Store) PreviousResult(applicationID, operation, requestID string) (CommandResult, bool, error) {
	if requestID == "" {
		return CommandResult{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _, err := s.read(s.snapshotPath(applicationID))
	if err != nil {
		return CommandResult{}, false, err
	}
	result, ok := data.Commands[commandKey(operation, requestID)]
	return result, ok, nil
}

func (s *Store) Events(applicationID string) ([]EventRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.eventPath)
	if errors.Is(err, os.ErrNotExist) {
		return []EventRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var result []EventRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event EventRecord
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("事件日志损坏: %w", err)
		}
		if event.ApplicationID == applicationID {
			result = append(result, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取事件日志: %w", err)
	}
	return result, nil
}

func (s *Store) commit(path string, data snapshotData, operation, requestID string) error {
	content, digest, err := encodeSnapshot(data)
	if err != nil {
		return err
	}
	if err := atomicWrite(path, content); err != nil {
		return err
	}
	s.sequence++
	event := EventRecord{Sequence: s.sequence, ApplicationID: data.Application.ID, Revision: data.Application.Revision, Operation: operation, RequestID: requestID, Status: data.Application.Status, At: time.Now().UTC(), SnapshotDigest: digest}
	return s.appendEvent(event)
}

func (s *Store) appendEvent(event EventRecord) error {
	file, err := os.OpenFile(s.eventPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	encoded, err := json.Marshal(event)
	if err == nil {
		_, err = file.Write(append(encoded, '\n'))
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("写入事件日志: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭事件日志: %w", closeErr)
	}
	return nil
}

func (s *Store) read(path string) (snapshotData, string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return snapshotData{}, "", ErrNotFound
	}
	if err != nil {
		return snapshotData{}, "", fmt.Errorf("读取快照: %w", err)
	}
	return decodeSnapshot(raw)
}

func (s *Store) snapshotPath(id string) string {
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, id)
	return filepath.Join(s.snapshots, safe+".json")
}

func (s *Store) findByTreeCodeLocked(treeCode string) ([]*domain.MigrationApplication, error) {
	if treeCode == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.snapshots)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.MigrationApplication, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, _, err := s.read(filepath.Join(s.snapshots, entry.Name()))
		if err != nil {
			return nil, err
		}
		if domain.NormalizeTreeCode(data.Application.TreeCode) == treeCode {
			copy, err := cloneApplication(data.Application)
			if err != nil {
				return nil, err
			}
			result = append(result, copy)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}

func (s *Store) findActiveDuplicateLocked(treeCode, excludeID string) (*domain.MigrationApplication, error) {
	matches, err := s.findByTreeCodeLocked(domain.NormalizeTreeCode(treeCode))
	if err != nil {
		return nil, err
	}
	for _, app := range matches {
		if app.ID != excludeID && domain.IsActiveStatus(app.Status) {
			return app, nil
		}
	}
	return nil, nil
}

func cloneApplication(input *domain.MigrationApplication) (*domain.MigrationApplication, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var output domain.MigrationApplication
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Store) recover() error {
	entries, err := os.ReadDir(s.snapshots)
	if err != nil {
		return fmt.Errorf("扫描快照: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if _, _, err := s.read(filepath.Join(s.snapshots, entry.Name())); err != nil {
			return fmt.Errorf("恢复 %s: %w", entry.Name(), err)
		}
	}
	file, err := os.Open(s.eventPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event EventRecord
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("恢复事件日志: %w", err)
		}
		if event.Sequence > s.sequence {
			s.sequence = event.Sequence
		}
	}
	return scanner.Err()
}
