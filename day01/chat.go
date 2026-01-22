package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
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

	// 2. 创建一个 Chain
	// 定义输入为 string (中文词), 输出为 *schema.Message
	// Eino 的泛型语法: NewChain[InputType, OutputType]()
	chain := compose.NewChain[string, *schema.Message]()

	// 3. 编排流程
	chain.
		// 简单的变换：将 string 转为 Prompt (这里演示简单的闭包转换)
		AppendLambda(compose.InvokableLambda(func(ctx context.Context, input string) ([]*schema.Message, error) {
			message := fmt.Sprintf("请将以下内容翻译成英文 %s", input)
			return []*schema.Message{schema.UserMessage(message)}, nil
		})).
		// 接入大模型
		AppendChatModel(chatModel)

	// 4. 编译 Chain
	// Compile 阶段会检查节点连接是否合法
	runnable, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("compile chain failed: %v", err)
	}

	// 5. 运行
	result, err := runnable.Invoke(ctx, "程序员")
	if err != nil {
		log.Fatalf("invoke failed: %v", err)
	}
	fmt.Printf("翻译结果: %v\n", result.Content)
}
