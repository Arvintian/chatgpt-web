package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/Arvintian/chatgpt-web/pkg/tokenizer"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ccache "github.com/karlseguin/ccache/v3"
	openai "github.com/sashabaranov/go-openai"
	"k8s.io/klog/v2"
)

const (
	ChatSessionTTL        = 30 * time.Minute
	ChatMinResponseTokens = 1000
)

type ChatService struct {
	client        *openai.Client
	store         *ccache.Cache[ChatMessage]
	params        ChatCompletionParams
	systemMessage openai.ChatCompletionMessage
}

type ChatCompletionParams openai.ChatCompletionRequest

type ChatMessageRequest struct {
	Prompt  string                    `json:"prompt"`
	Options ChatMessageRequestOptions `json:"options"`
}

type ChatMessageRequestOptions struct {
	Name            string `json:"name"`
	ParentMessageId string `json:"parentMessageId"`
}

type ChatMessage struct {
	ID              string                              `json:"id"`
	Text            string                              `json:"text"`
	Role            string                              `json:"role"`
	Name            string                              `json:"name"`
	Delta           string                              `json:"delta"`
	Detail          openai.ChatCompletionStreamResponse `json:"detail"`
	ParentMessageId string                              `json:"parentMessageId"`
}

func NewChatService(apiKey string, baseURL string, socksProxy string, params ChatCompletionParams) (*ChatService, error) {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	klog.Infof("use openai base url: %s", config.BaseURL)
	if socksProxy != "" {
		proxyUrl, err := url.Parse(socksProxy) //socks5://user:password@127.0.0.1:1080
		if err != nil {
			return nil, err
		}
		config.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
			},
		}
		klog.Infof("use sock proxy: %s", proxyUrl)
	}
	currentDate := time.Now().Format("2006-01-02")
	chat := ChatService{
		client: openai.NewClientWithConfig(config),
		params: params,
		store:  ccache.New(ccache.Configure[ChatMessage]()),
		systemMessage: openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf(`You are ChatGPT, a large language model trained by OpenAI. Answer as concisely as possible.\nKnowledge cutoff: 2021-09-01\nCurrent date: %s`, currentDate),
		},
	}
	return &chat, nil
}

func (chat *ChatService) ChatProcess(ctx *gin.Context) {
	payload := ChatMessageRequest{}
	if err := ctx.BindJSON(&payload); err != nil {
		klog.Error(err)
		ctx.JSON(200, gin.H{
			"status":  "Fail",
			"message": fmt.Sprintf("%v", err),
			"data":    nil,
		})
		return
	}

	messageID := uuid.New().String()

	message := ChatMessage{
		ID:              messageID,
		Role:            openai.ChatMessageRoleUser,
		Text:            payload.Prompt,
		ParentMessageId: payload.Options.ParentMessageId,
	}

	chat.store.Set(messageID, message, ChatSessionTTL)

	result := ChatMessage{
		ID:              uuid.New().String(),
		Role:            openai.ChatMessageRoleAssistant,
		Text:            "",
		ParentMessageId: messageID,
	}

	messages, numTokens, err := chat.buildMessage(payload)
	if err != nil {
		ctx.JSON(200, gin.H{
			"status":  "Fail",
			"message": fmt.Sprintf("%v", err),
			"data":    nil,
		})
	}

	//klog.Infof("send message %d tokens, set call model %d max tokens", numTokens, chat.params.MaxTokens-numTokens)

	stream, err := chat.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:            chat.params.Model,
		Messages:         messages,
		MaxTokens:        chat.params.MaxTokens - numTokens,
		Temperature:      chat.params.Temperature,
		PresencePenalty:  chat.params.PresencePenalty,
		FrequencyPenalty: chat.params.FrequencyPenalty,
		TopP:             1,
		Stream:           true,
	})
	if err != nil {
		klog.Error(err)
		ctx.JSON(200, gin.H{
			"status":  "Fail",
			"message": fmt.Sprintf("%v", err),
			"data":    nil,
		})
		return
	}
	defer stream.Close()

	resp := stream.GetResponse()
	if resp.StatusCode != 200 {
		bts, _ := ioutil.ReadAll(resp.Body)
		ctx.JSON(200, gin.H{
			"status":  "Fail",
			"message": fmt.Sprintf("%v", string(bts)),
			"data":    nil,
		})
		return
	}

	firstChunk := true
	ctx.Header("Content-type", "application/octet-stream")
	for {
		rsp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			chat.store.Set(result.ID, result, ChatSessionTTL)
			return
		}

		if err != nil {
			klog.Error(err)
			ctx.JSON(200, gin.H{
				"status":  "Fail",
				"message": fmt.Sprintf("OpenAI Event Error %v", err),
				"data":    nil,
			})
			return
		}

		if rsp.ID != "" {
			result.ID = rsp.ID
		}

		if len(rsp.Choices) > 0 {
			content := rsp.Choices[0].Delta.Content
			result.Delta = content
			result.Text += content
			result.Detail = rsp
		}

		bts, err := json.Marshal(result)
		if err != nil {
			klog.Error(err)
			ctx.JSON(200, gin.H{
				"status":  "Fail",
				"message": fmt.Sprintf("OpenAI Event Marshal Error %v", err),
				"data":    nil,
			})
			return
		}

		out := bytes.Buffer{}
		if !firstChunk {
			out.Write([]byte("\n"))
		}
		out.Write(bts)

		if _, err := ctx.Writer.Write(out.Bytes()); err != nil {
			klog.Error(err)
			return
		}

		ctx.Writer.Flush()
		firstChunk = false
	}
}

func (chat *ChatService) buildMessage(payload ChatMessageRequest) ([]openai.ChatCompletionMessage, int, error) {
	parentMessageId := payload.Options.ParentMessageId
	messages := []openai.ChatCompletionMessage{}
	messages = append(messages, chat.systemMessage)
	systemMessageOffset := len(messages)
	chatMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: payload.Prompt,
		Name:    payload.Options.Name,
	}
	nextMessages := append(messages, chatMessage)
	numTokens, err := tokenizer.GetTokenCount(chatMessage, chat.params.Model)
	if err != nil {
		return nil, 0, err
	}
	numTokens += 59 // add sum the system message token count
	for {
		messages = nextMessages
		if parentMessageId == "" {
			break
		}
		parentMessage, ok := chat.getMessageByID(parentMessageId)
		if !ok {
			break
		}
		parentCompletioMessage := openai.ChatCompletionMessage{
			Role:    parentMessage.Role,
			Content: parentMessage.Text,
			Name:    parentMessage.Name,
		}
		tokenCount, err := tokenizer.GetTokenCount(parentCompletioMessage, chat.params.Model)
		if err != nil {
			return nil, 0, err
		}
		if (numTokens + tokenCount) >= (chat.params.MaxTokens - ChatMinResponseTokens) {
			break
		}
		numTokens += tokenCount
		nextMessages = append(nextMessages[:systemMessageOffset], append([]openai.ChatCompletionMessage{parentCompletioMessage}, nextMessages[systemMessageOffset:]...)...)
		parentMessageId = parentMessage.ParentMessageId
	}
	return messages, numTokens, nil
}

func (chat *ChatService) getMessageByID(id string) (ChatMessage, bool) {
	item := chat.store.Get(id)
	if item == nil {
		return ChatMessage{}, false
	}
	if item.Expired() {
		return ChatMessage{}, false
	}
	return item.Value(), true
}
