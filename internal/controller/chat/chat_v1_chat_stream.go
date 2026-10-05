package chat

import (
	"SuperBizAgent/api/chat/v1"
	"SuperBizAgent/internal/ai/agent/chat_pipeline"
	"SuperBizAgent/internal/ai/agent/memory"
	"SuperBizAgent/pkg/log_call_back"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ChatStream(ctx context.Context, req *v1.ChatStreamReq) (res *v1.ChatStreamRes, err error) {
	id, msg := req.Id, req.Question

	ctx = context.WithValue(ctx, "client_id", req.Id)
	client, err := c.service.Create(ctx, g.RequestFromCtx(ctx))
	if err != nil {
		return nil, err
	}

	userID := getUserID(ctx)
	memMgr := memory.GetDefaultMemoryManager()
	userMessage := &chat_pipeline.UserMessage{
		ID:      id,
		Query:   msg,
		History: memMgr.GetHistory(ctx, id, userID, msg),
	}

	runner, err := chat_pipeline.BuildChatAgent(ctx)
	sr, err := runner.Stream(ctx, userMessage, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		client.SendToClient("error", err.Error())
		return nil, err
	}
	defer sr.Close()

	var fullResponse strings.Builder
	defer func() {
		completeResponse := fullResponse.String()
		if completeResponse != "" && memMgr != nil {
			memMgr.RecordInteractionAsync(id, userID, msg, completeResponse, 0, 0, "")
		}
	}()

	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			client.SendToClient("done", "Stream completed")
			return &v1.ChatStreamRes{}, nil
		}
		if err != nil {
			client.SendToClient("error", err.Error())
			return &v1.ChatStreamRes{}, nil
		}
		if chunk.Content == "" {
			continue
		}
		fullResponse.WriteString(chunk.Content)
		payload, _ := json.Marshal(map[string]string{"content": chunk.Content})
		client.SendToClient("message", string(payload))
	}
}
