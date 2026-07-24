package jsonl

import (
	"path/filepath"
)

func (s *Store) sessionIndexPath() string {
	return filepath.Join(s.root, "session_index.jsonl")
}

func (s *Store) threadJSONLPath(threadID string) string {
	return filepath.Join(s.root, "threads", threadID+".jsonl")
}

func (s *Store) threadSeqPath(threadID string) string {
	return filepath.Join(s.root, "threads", threadID+".seq")
}

func (s *Store) threadLockPath(threadID string) string {
	return filepath.Join(s.root, "threads", threadID+".lock")
}

func (s *Store) interruptDir(threadID string) string {
	return filepath.Join(s.root, "interrupt", threadID)
}

func interruptFlagPath(root, threadID, turnID string) string {
	return filepath.Join(root, "interrupt", threadID, turnID+".flag")
}
