package chat

import (
	"SuperBizAgent/api/chat/v1"
	"SuperBizAgent/internal/ai/agent/chat_pipeline"
	"SuperBizAgent/internal/ai/agent/memory"
	"SuperBizAgent/pkg/log_call_back"
	"context"

	"github.com/cloudwego/eino/compose"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	id, msg := req.Id, req.Question
	userID := getUserID(ctx)

	memMgr := memory.GetDefaultMemoryManager()
	userMessage := &chat_pipeline.UserMessage{
		ID:      id,
		Query:   msg,
		History: memMgr.GetHistory(ctx, id, userID, msg),
	}

	runner, err := chat_pipeline.BuildChatAgent(ctx)
	if err != nil {
		return nil, err
	}

	out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return nil, err
	}

	if memMgr != nil {
		memMgr.RecordInteractionAsync(id, userID, msg, out.Content, 0, 0, "")
	}

	return &v1.ChatRes{Answer: out.Content}, nil
}
