package main

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// 1. 定义 Prompt Template
	// 我们定义了两个变量：{language} 和 {text}
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("你是一个专业的翻译官，请将用户输入的文本翻译成 {language}。只返回翻译结果。"),
		schema.UserMessage("{text}"),
	)

	// 2. 初始化 ChatModel (开启流式处理通常不需要特殊配置，取决于调用方法)
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("init model failed: %v", err)
	}

	// 3. 运行模板生成具体消息
	// 模拟用户输入：翻译成“法文”
	variables := map[string]any{
		"language": "英文",
		"text":     "坚持学习 Golang 是一个明智的选择。",
	}
	messages, err := template.Format(ctx, variables)
	if err != nil {
		log.Fatalf("format prompt failed: %v", err)
	}

	// 4. 流式调用 (Stream)
	// 在生产环境中，流式输出能极大减少用户的等待焦虑感
	fmt.Printf("[翻译结果]: ")
	reader, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("stream call failed: %v", err)
	}

	// 5. 处理流式返回的 Reader
	defer reader.Close()
	for {
		chunk, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("read stream failed: %v", err)
		}
		// 打印每一帧内容
		fmt.Print(chunk.Content)
	}
	fmt.Println("\n--- 任务完成 ---")
}
