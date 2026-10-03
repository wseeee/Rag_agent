package tika

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestExtractText(t *testing.T) {
	client := NewClient("http://127.0.0.1:9998")

	content := "Hello Apache Tika. 这是一个针对 SuperBizAgent 的纯文本与富媒体抽取测试。"
	reader := strings.NewReader(content)

	ctx := context.Background()
	text, err := client.ExtractText(ctx, reader, "test.txt")
	if err != nil {
		t.Logf("Tika 服务不可用或测试跳过: %v", err)
		return
	}

	fmt.Println(text)
	if !strings.Contains(text, "SuperBizAgent") {
		t.Fatalf("提取文本未包含预期内容: got %q", text)
	}
}
