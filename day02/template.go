package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	_ = godotenv.Load()
	// 1. 初始化 ChatModel 组件 (以 OpenAI 为例)
	// Eino 通过泛型保证了输入输出的类型安全
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("init model failed: %v", err)
	}

	// 2. 定义 Prompt Template
	// 我们定义两个变量：{style} 和 {text}
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("你是一个专业的翻译官，请使用{style}的语气进行翻译。"),
		schema.UserMessage("{text}"),
	)

	// 3. 构建 Chain
	// 注意：输入变成了 map[string]any，用于承载多个变量
	chain := compose.NewChain[map[string]any, *schema.Message]()
	chain.
		AppendChatTemplate(template).
		AppendChatModel(chatModel)

	runnable, _ := chain.Compile(ctx)

	// 4. 发起流式调用 (Stream)
	// 输入不再是简单的 string，而是模板变量 map
	input := map[string]any{
		"style": "好莱坞电影台词",
		"text":  "今天的代码写得真不错。",
	}

	// 使用 Stream 方法而非 Invoke
	streamReader, err := runnable.Stream(ctx, input)
	if err != nil {
		log.Fatalf("stream call failed: %v", err)
	}
	defer streamReader.Close() // 记得关闭流

	// 5. 循环读取流数据
	fmt.Print("Agent 回复: ")
	for {
		chunk, err := streamReader.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break // 读取完毕
			}
			log.Fatalf("recv failed: %v", err)
		}
		// 实时打印每个碎片内容
		fmt.Print(chunk.Content)
	}
	fmt.Println("\n--- 任务完成 ---")
}
