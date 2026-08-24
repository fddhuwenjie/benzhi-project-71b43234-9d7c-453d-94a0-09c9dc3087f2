package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

func encodeSnapshot(data snapshotData) ([]byte, string, error) {
	data.Format = snapshotFormat
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, "", fmt.Errorf("编码快照: %w", err)
	}
	digest := domain.Digest(string(raw))
	envelope, err := json.Marshal(snapshotEnvelope{Digest: digest, Data: raw})
	if err != nil {
		return nil, "", fmt.Errorf("编码快照封装: %w", err)
	}
	return append(envelope, '\n'), digest, nil
}

func decodeSnapshot(raw []byte) (snapshotData, string, error) {
	var envelope snapshotEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return snapshotData{}, "", fmt.Errorf("解析快照封装: %w", err)
	}
	actual := domain.Digest(string(envelope.Data))
	if envelope.Digest == "" || envelope.Digest != actual {
		return snapshotData{}, "", fmt.Errorf("快照摘要校验失败")
	}
	var data snapshotData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return snapshotData{}, "", fmt.Errorf("解析快照数据: %w", err)
	}
	if data.Format != snapshotFormat || data.Application == nil {
		return snapshotData{}, "", fmt.Errorf("快照格式不受支持")
	}
	if data.Commands == nil {
		data.Commands = make(map[string]CommandResult)
	}
	return data, actual, nil
}

func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时快照: %w", err)
	}
	tempName := temp.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("设置快照权限: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("写入临时快照: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("同步临时快照: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("关闭临时快照: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("替换快照: %w", err)
	}
	remove = false
	dirHandle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("打开数据目录: %w", err)
	}
	defer dirHandle.Close()
	if err := dirHandle.Sync(); err != nil {
		return fmt.Errorf("同步数据目录: %w", err)
	}
	return nil
}
