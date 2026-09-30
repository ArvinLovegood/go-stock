package tools

// data_tools_wrapper_info_test.go — DataToolWrapper.Info 缓存的守护测试。

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestDataToolWrapperInfoCached Info 多次调用应返回同一指针（schema 只构建一次）。
func TestDataToolWrapperInfoCached(t *testing.T) {
	w := NewDataToolWrapper("TestTool", "测试工具", map[string]*schema.ParameterInfo{
		"code": {Type: "string", Desc: "代码", Required: true},
	}, func(args string) (string, error) { return "ok", nil })

	i1, err := w.Info(context.Background())
	if err != nil || i1 == nil {
		t.Fatalf("Info 调用失败: %v", err)
	}
	i2, err := w.Info(context.Background())
	if err != nil {
		t.Fatalf("Info 第二次调用失败: %v", err)
	}
	if i1 != i2 {
		t.Error("Info 应缓存并返回同一指针")
	}
	if i1.Name != "TestTool" || i1.Desc != "测试工具" {
		t.Errorf("Info 内容不正确: %+v", i1)
	}
}

// TestDataToolWrapperInfoConcurrent 并发调用 Info 不应出现数据竞争（go test -race）。
func TestDataToolWrapperInfoConcurrent(t *testing.T) {
	w := NewDataToolWrapper("ConcurrentTool", "并发测试", nil,
		func(args string) (string, error) { return "", nil })

	var wg sync.WaitGroup
	infos := make([]*schema.ToolInfo, 32)
	for i := range infos {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			info, _ := w.Info(context.Background())
			infos[idx] = info
		}(i)
	}
	wg.Wait()
	for i := 1; i < len(infos); i++ {
		if infos[i] != infos[0] {
			t.Fatal("并发下 Info 返回了不同实例")
		}
	}
}
