package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	_ = godotenv.Load()
	var handlers []callbacks.Handler

	callbacks.AppendGlobalHandlers(handlers...)

	schema.RegisterName[myState]("state")
	runner, err := composeGraph[map[string]any, *schema.Message](
		ctx,
		newChatTemplate(ctx),
		newChatToolModel(ctx),
		newToolsNode(ctx),
		newCheckPointStore(ctx),
	)
	if err != nil {
		log.Fatal(err)
	}

	var history []*schema.Message

	for {
		result, err := runner.Invoke(ctx, map[string]any{"location": "北京"}, compose.WithCheckPointID("1"), compose.WithStateModifier(func(ctx context.Context, path compose.NodePath, state any) error {
			state.(*myState).history = history
			return nil
		}))
		if err == nil {
			fmt.Printf("final result: %s", result.Content)
			break
		}

		info, ok := compose.ExtractInterruptInfo(err)
		if !ok {
			log.Fatal(err)
		}

		history = info.State.(*myState).history
		for i, tc := range history[len(history)-1].ToolCalls {
			fmt.Printf("will call tool: %s, arguments: %s\n", tc.Function.Name, tc.Function.Arguments)
			fmt.Print("Are the arguments as expected? (y/n): ")
			var response string
			_, _ = fmt.Scanln(&response)

			if strings.ToLower(response) == "n" {
				fmt.Print("Please enter the modified arguments: ")
				scanner := bufio.NewScanner(os.Stdin)
				var newArguments string
				if scanner.Scan() {
					newArguments = scanner.Text()
				}

				// Update the tool call arguments
				history[len(history)-1].ToolCalls[i].Function.Arguments = newArguments
				fmt.Printf("Updated arguments to: %s\n", newArguments)
			}
		}
	}
}

func newChatTemplate(_ context.Context) prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage("穿搭助手，根据地区天气提供穿搭建议"),
		schema.UserMessage("帮我看看{location}的天气，然后告诉我适合穿什么。"),
	)
}

func newChatToolModel(ctx context.Context) model.BaseChatModel {
	cm, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
	if err != nil {
		log.Fatal(err)
	}

	tools := getTools()
	var toolsInfo []*schema.ToolInfo
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		toolsInfo = append(toolsInfo, info)
	}

	err = cm.BindTools(toolsInfo)
	if err != nil {
		log.Fatal(err)
	}
	return cm
}

func newToolsNode(ctx context.Context) *compose.ToolsNode {
	tools := getTools()

	tn, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		log.Fatal(err)
	}
	return tn
}

func newCheckPointStore(ctx context.Context) compose.CheckPointStore {
	return &myStore{buf: make(map[string][]byte)}
}

type myState struct {
	history []*schema.Message
}

func composeGraph[I, O any](ctx context.Context, tpl prompt.ChatTemplate, cm model.BaseChatModel, tn *compose.ToolsNode, store compose.CheckPointStore) (compose.Runnable[I, O], error) {
	g := compose.NewGraph[I, O](compose.WithGenLocalState(func(ctx context.Context) *myState {
		return &myState{}
	}))
	err := g.AddChatTemplateNode(
		"ChatTemplate",
		tpl,
	)
	if err != nil {
		return nil, err
	}
	err = g.AddChatModelNode(
		"ChatModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}
	err = g.AddToolsNode("ToolsNode", tn, compose.WithStatePreHandler(func(ctx context.Context, in *schema.Message, state *myState) (*schema.Message, error) {
		return state.history[len(state.history)-1], nil
	}))
	if err != nil {
		return nil, err
	}

	err = g.AddEdge(compose.START, "ChatTemplate")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("ChatTemplate", "ChatModel")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("ToolsNode", "ChatModel")
	if err != nil {
		return nil, err
	}
	err = g.AddBranch("ChatModel", compose.NewGraphBranch(func(ctx context.Context, in *schema.Message) (endNode string, err error) {
		if len(in.ToolCalls) > 0 {
			return "ToolsNode", nil
		}
		return compose.END, nil
	}, map[string]bool{"ToolsNode": true, compose.END: true}))
	if err != nil {
		return nil, err
	}
	return g.Compile(
		ctx,
		compose.WithCheckPointStore(store),
		compose.WithInterruptBeforeNodes([]string{"ToolsNode"}),
	)
}

type WeatherArgs struct {
	Location string `json:"location" jsonschema:"description=城市名,比如北京"`
}

type WeatherReply struct {
	Weather string `json:"weather"`
}

func getTools() []tool.BaseTool {
	weatherTool := utils.NewTool(&schema.ToolInfo{
		Name: "get_weather",
		Desc: "根据城市名查询天气",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"location": {
				Desc: "城市名",
				Type: schema.String,
			},
		}),
	}, func(ctx context.Context, args *WeatherArgs) (*WeatherReply, error) {
		// 这里接入实际的 API，现在模拟返回
		return &WeatherReply{Weather: fmt.Sprintf("%s今天晴转多云，0到-12度", args.Location)}, nil
	})

	return []tool.BaseTool{
		weatherTool,
	}
}

type myStore struct {
	buf map[string][]byte
}

func (m *myStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	data, ok := m.buf[checkPointID]
	return data, ok, nil
}

func (m *myStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	m.buf[checkPointID] = checkPoint
	return nil
}
