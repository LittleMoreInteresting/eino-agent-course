package main

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

type AssistantState struct {
	Intent string // 识别出的意图
}

func main() {
	ctx := context.Background()
	_ = godotenv.Load()
	runnable, err2 := Buildgraph(ctx)
	if err2 != nil {
		log.Fatalf("Failed to build graph: %v", err2)
	}

	// 测试：翻译意图
	fmt.Println("测试翻译功能:")
	res1, err := runnable.Invoke(ctx, "帮我翻译：Hello World")
	if err != nil {
		log.Printf("Error in translation: %v", err)
	} else {
		fmt.Printf("翻译结果: %s\n", res1)
	}

	// 测试：闲聊意图
	fmt.Println("\n测试闲聊功能:")
	res2, err := runnable.Invoke(ctx, "今天天气不错")
	if err != nil {
		log.Printf("Error in chat: %v", err)
	} else {
		fmt.Printf("闲聊结果: %s\n", res2)
	}
}
func Buildgraph(ctx context.Context) (r compose.Runnable[string, *schema.Message], err error) {
	const (
		LambdaTranslate  = "translate"
		CustomChatModel2 = "CustomChatModel2"
		LambdaChat       = "chat"
	)
	g := compose.NewGraph[string, *schema.Message]()
	_ = g.AddLambdaNode(LambdaTranslate, compose.InvokableLambda(newLambda))
	customChatModel2KeyOfChatModel, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddChatModelNode(CustomChatModel2, customChatModel2KeyOfChatModel)
	_ = g.AddLambdaNode(LambdaChat, compose.InvokableLambda(newLambda1))
	_ = g.AddEdge(CustomChatModel2, compose.END)
	_ = g.AddEdge(LambdaTranslate, CustomChatModel2)
	_ = g.AddEdge(LambdaChat, CustomChatModel2)
	_ = g.AddBranch(compose.START, compose.NewGraphBranch(newBranch, map[string]bool{LambdaTranslate: true, LambdaChat: true}))
	r, err = g.Compile(ctx, compose.WithGraphName("graph"))
	if err != nil {
		return nil, err
	}
	return r, err
}

// newChatModel component initialization function of node 'CustomChatModel2' in graph 'graph'
func newChatModel(ctx context.Context) (cm model.BaseChatModel, err error) {
	return ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
}

// newLambda component initialization function of node 'Lambda1' in graph 'graph'
func newLambda(ctx context.Context, input any) (output any, err error) {
	message := input.(string)
	return []*schema.Message{
		schema.SystemMessage("你是一个专业的翻译官。请将用户输入的内容准确翻译成目标语言。"),
		schema.UserMessage(message),
	}, nil
}

// newLambda1 component initialization function of node 'Lambda3' in graph 'graph'
func newLambda1(ctx context.Context, input any) (output any, err error) {
	message := input.(string)
	return []*schema.Message{
		schema.SystemMessage("你是一个有用的助手，请友好地回复用户的问题。"),
		schema.UserMessage(message),
	}, nil
}

// newBranch branch initialization method of node 'start' in graph 'graph'
func newBranch(ctx context.Context, input string) (endNode string, err error) {
	if strings.Contains(input, "翻译") || strings.Contains(strings.ToLower(input), "translate") {
		return "translate", nil
	} else {
		return "chat", nil
	}
}
