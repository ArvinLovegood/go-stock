package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AgentRunSnapshot 是运行恢复所需的元信息和轻量事件历史。
type AgentRunSnapshot struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionId"`
	Question   string          `json:"question"`
	AIConfigID int             `json:"aiConfigId,omitempty"`
	Mode       Mode            `json:"mode"`
	Phase      string          `json:"phase"`
	State      AgentRunState   `json:"state"`
	StartedAt  time.Time       `json:"startedAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	Budget     AgentRunBudget  `json:"budget"`
	ToolCalls  int             `json:"toolCalls"`
	Events     []AgentRunEvent `json:"events,omitempty"`
}

// AgentRunEvent 是恢复和诊断所需的轻量事件，不保存完整工具结果，避免快照膨胀或泄漏大段原始数据。
type AgentRunEvent struct {
	Sequence      int       `json:"sequence"`
	Type          string    `json:"type"`
	Name          string    `json:"name,omitempty"`
	Status        string    `json:"status,omitempty"`
	At            time.Time `json:"at"`
	ArgPreview    string    `json:"argPreview,omitempty"`
	ResultPreview string    `json:"resultPreview,omitempty"`
}

type AgentRunCheckpointStore struct {
	dir string
	mu  sync.Mutex
}

func NewAgentRunCheckpointStore(rootDir string) *AgentRunCheckpointStore {
	if strings.TrimSpace(rootDir) == "" || rootDir == "." {
		return &AgentRunCheckpointStore{}
	}
	return &AgentRunCheckpointStore{dir: filepath.Join(rootDir, "memory", ".agent-runs")}
}

func (s *AgentRunCheckpointStore) Save(snapshot AgentRunSnapshot) error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("运行快照目录不可用")
	}
	if !validRunID(snapshot.ID) {
		return fmt.Errorf("运行 ID 不安全")
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化运行快照失败: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("创建运行快照目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, ".run-*.tmp")
	if err != nil {
		return fmt.Errorf("创建运行快照临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入运行快照失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("同步运行快照失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭运行快照失败: %w", err)
	}
	if err := os.Rename(tmpPath, filepath.Join(s.dir, snapshot.ID+".json")); err != nil {
		return fmt.Errorf("替换运行快照失败: %w", err)
	}
	return nil
}

func (s *AgentRunCheckpointStore) Load(id string) (AgentRunSnapshot, error) {
	if s == nil || s.dir == "" || !validRunID(id) {
		return AgentRunSnapshot{}, fmt.Errorf("运行快照参数无效")
	}
	b, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return AgentRunSnapshot{}, err
	}
	var snapshot AgentRunSnapshot
	if err := json.Unmarshal(b, &snapshot); err != nil {
		return AgentRunSnapshot{}, fmt.Errorf("解析运行快照失败: %w", err)
	}
	if snapshot.ID != id {
		return AgentRunSnapshot{}, fmt.Errorf("运行快照 ID 不匹配")
	}
	return snapshot, nil
}

// ListIncomplete 返回仍可能需要恢复的运行，按最近更新时间倒序排列。
func (s *AgentRunCheckpointStore) ListIncomplete() ([]AgentRunSnapshot, error) {
	if s == nil || s.dir == "" {
		return nil, fmt.Errorf("运行快照目录不可用")
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []AgentRunSnapshot{}, nil
		}
		return nil, err
	}
	result := make([]AgentRunSnapshot, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		snapshot, err := s.Load(id)
		if err != nil {
			continue
		}
		switch snapshot.State {
		case AgentRunCreated, AgentRunRunning, AgentRunWaiting, AgentRunFailed, AgentRunCanceled:
			result = append(result, snapshot)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result, nil
}

func (s *AgentRunCheckpointStore) Delete(id string) error {
	if s == nil || s.dir == "" || !validRunID(id) {
		return fmt.Errorf("运行快照参数无效")
	}
	if err := os.Remove(filepath.Join(s.dir, id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validRunID(id string) bool {
	return id != "" && !strings.ContainsAny(id, `/\\`) && id != "." && id != ".."
}
