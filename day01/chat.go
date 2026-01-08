package main

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// 1. 初始化 ChatModel 组件
	// 这里以 OpenAI 适配器为例，配置 APIKey 和模型名称
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("failed to init chat model: %v", err)
	}

	// 2. 构造消息
	messages := []*schema.Message{
		schema.SystemMessage("你是一名资深的 Golang 专家，负责引导开发者学习 Eino 框架。"),
		schema.UserMessage("请用一句话介绍 Eino 的核心优势。"),
	}

	// 3. 调用模型生成响应
	// Eino 统一了组件接口，所有 Model 都遵循 Generate 方法
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("chat model generate failed: %v", err)
	}

	// 4. 打印结果
	fmt.Printf("\n[AI]: %s\n", response.Content)
}
