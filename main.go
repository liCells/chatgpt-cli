package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"golang.design/x/clipboard"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	inf "github.com/fzdwx/infinite"
	"github.com/fzdwx/infinite/components/input/text"
	"github.com/fzdwx/infinite/theme"
)

const (
	GREEN  = "\033[32m"
	PURPLE = "\033[;35m"
)

var (
	commandInterpreter       string
	token                    string
	proxy                    string
	copyToClipboardByDefault bool
	h                        bool
)

func main() {
	flag.StringVar(&commandInterpreter, "c", "bash", "command interpreter")
	flag.StringVar(&token, "t", "", "openAI KEY")
	flag.StringVar(&proxy, "p", "", "proxy address")
	flag.BoolVar(&copyToClipboardByDefault, "C", false, "copy to clipboard by default")
	flag.BoolVar(&h, "h", false, "help")

	flag.Usage = usage

	flag.Parse()
	if h {
		flag.Usage()
		return
	}

	i := inf.NewText(
		text.WithPrompt("Enter your question?"),
		text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
	)

	question, _ := i.Display()

	if len(question) == 0 {
		fmt.Print("No question, exit.\n")
		return
	}

	chatCompletionMessage := ChatCompletionMessage{
		Model: "gpt-3.5-turbo",
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "你充当 Linux 终端。我输入问题，您回复应该使用什么命令。\\n我希望您只在唯一的代码块内回复终端代码，而不是其他任何内容。不要写解释。除非我指示您这样做，否则不要键入命令。在我建议您的时候, 请在上一命令的基础上进行改进。\\n我的第一个命令是 列出当前文件夹路径",
			},
			{
				Role:    "user",
				Content: question,
			},
		},
	}

	chatCompletionResponse := completionMessages(chatCompletionMessage)

	ask(chatCompletionResponse, chatCompletionMessage, chatCompletionResponse.Choices[0].Message.Content)
}

func usage() {
	fmt.Print("Example: -c zsh -t openAI_APIKey -p http://127.0.0.1:7890 -C\n\n")
	flag.PrintDefaults()
}

func ask(chatCompletionResponse ChatCompletionResponse, chatCompletionMessage ChatCompletionMessage, command string) {
	fmt.Println(GREEN, command)

	if copyToClipboardByDefault {
		writeClipboard(command)
	}
	i := inf.NewText(
		text.WithPrompt("Do you want to execute this command?"),
		text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
		text.WithDefaultValue("Y/n/s(suggestion)/e(explain)/c(Copy to Clipboard)"),
	)
	whether, _ := i.Display()

	if strings.EqualFold(whether, "y") {
		executeCommand(command)
	} else if strings.EqualFold(whether, "c") {
		writeClipboard(command)
	} else if strings.EqualFold(whether, "e") {
		chatCompletionMessage.Messages = append(chatCompletionMessage.Messages, ChatMessage{
			Role:    "user",
			Content: "解释" + command,
		})
		chatCompletionResponse = completionMessages(chatCompletionMessage)
		fmt.Println(PURPLE, "\nExplain: ", chatCompletionResponse.Choices[0].Message.Content)

		ask(chatCompletionResponse, chatCompletionMessage, command)
	} else if strings.EqualFold(whether, "s") {
		chatCompletionMessage.Messages = append(chatCompletionMessage.Messages, ChatMessage{
			Role:    "user",
			Content: "建议" + askSuggestion(),
		})
		chatCompletionResponse = completionMessages(chatCompletionMessage)

		ask(chatCompletionResponse, chatCompletionMessage, chatCompletionResponse.Choices[0].Message.Content)
	}
}

func writeClipboard(command string) {
	err := clipboard.Init()
	if err != nil {
		panic(err)
	}
	clipboard.Write(clipboard.FmtText, []byte(command))
}

func askSuggestion() string {
	i := inf.NewText(
		text.WithPrompt("Enter your suggestion:"),
		text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
	)

	suggestion, _ := i.Display()

	if len(suggestion) == 0 {
		askSuggestion()
	}
	return suggestion
}

func executeCommand(command string) {
	cmd := exec.Command(commandInterpreter, "-c", command)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println("\nExecuting command:", command)

	err := cmd.Run()
	if err != nil {
		fmt.Printf("cmd.Run: %s failed: %s\n", err, err)
	}
}

func completionMessages(message ChatCompletionMessage) ChatCompletionResponse {
	body, _ := json.Marshal(message)
	req, _ := http.NewRequest(
		"POST",
		"https://api.openai.com/v1/chat/completions",
		strings.NewReader(string(body)))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	var clt http.Client
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			panic(err)
		}
		clt = http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
	} else {
		clt = http.Client{}
	}

	resp, err := clt.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	body, _ = io.ReadAll(resp.Body)
	var chatCompletionResponse ChatCompletionResponse
	_ = json.Unmarshal(body, &chatCompletionResponse)
	if len(chatCompletionResponse.Choices) == 0 {
		panic("No response, retry later.")
	}
	return chatCompletionResponse
}

type ChatCompletionMessage struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
