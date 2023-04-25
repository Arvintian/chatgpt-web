package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Arvintian/chatgpt-web/pkg/tokenizer"
	"github.com/Arvintian/chatgpt-web/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ccache "github.com/karlseguin/ccache/v3"
	openai "github.com/sashabaranov/go-openai"
	"k8s.io/klog/v2"
)

const (
	ChatPrimedTokens = 2
)

type ChatService struct {
	client *openai.Client
	store  *ccache.Cache[ChatMessage]
	params ChatCompletionParams
}

type ChatCompletionParams struct {
	Model                 string        `json:"model"`
	MaxTokens             int           `json:"max_tokens,omitempty"`
	Temperature           float32       `json:"temperature,omitempty"`
	PresencePenalty       float32       `json:"presence_penalty,omitempty"`
	FrequencyPenalty      float32       `json:"frequency_penalty,omitempty"`
	ChatSessionTTL        time.Duration `json:"chat_session_ttl"`
	ChatMinResponseTokens int           `json:"chat_min_response_tokens"`
}

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
	TokenCount      int                                 `json:"tokenCount"`
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
	chat := ChatService{
		client: openai.NewClientWithConfig(config),
		params: params,
		store:  ccache.New(ccache.Configure[ChatMessage]()),
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

	result := ChatMessage{
		ID:              uuid.New().String(),
		Role:            openai.ChatMessageRoleAssistant,
		Text:            "",
		ParentMessageId: messageID,
	}

	messages, numTokens, tokenCount, err := chat.buildMessage(payload)
	if err != nil {
		ctx.JSON(200, gin.H{
			"status":  "Fail",
			"message": fmt.Sprintf("%v", err),
			"data":    nil,
		})
		return
	}

	message.TokenCount = tokenCount
	chat.store.Set(messageID, message, chat.params.ChatSessionTTL)

	//klog.Infof("send message %d tokens, set completion %d max tokens", numTokens, chat.params.MaxTokens-numTokens)

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
		bts, _ := io.ReadAll(resp.Body)
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
			go func() {
				tokenCount, err := tokenizer.GetTokenCount(openai.ChatCompletionMessage{
					Role:    result.Role,
					Content: result.Text,
					Name:    result.Name,
				}, chat.params.Model)
				if err != nil {
					klog.Error(err)
				}
				result.TokenCount = tokenCount
				chat.store.Set(result.ID, result, chat.params.ChatSessionTTL)
			}()
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
			if len(content) > 0 {
				result.Text += content
			}
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

		if !firstChunk {
			ctx.Writer.Write([]byte("\n"))
		} else {
			firstChunk = false
		}

		if _, err := ctx.Writer.Write(bts); err != nil {
			klog.Error(err)
			return
		}

		ctx.Writer.Flush()
	}
}

func (chat *ChatService) buildMessage(payload ChatMessageRequest) ([]openai.ChatCompletionMessage, int, int, error) {
	parentMessageId := payload.Options.ParentMessageId
	messages := []openai.ChatCompletionMessage{}
	tokenCount := 0
	var err error
	if len(payload.Prompt) > 0 {
		chatMessage := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: payload.Prompt,
			Name:    payload.Options.Name,
		}
		messages = append(messages, chatMessage)
		tokenCount, err = tokenizer.GetTokenCount(chatMessage, chat.params.Model)
		if err != nil {
			return nil, 0, 0, err
		}
		if tokenCount >= (chat.params.MaxTokens - chat.params.ChatMinResponseTokens) {
			return nil, 0, 0, fmt.Errorf("this model's maximum context length is %d tokens. you requested %d tokens in the messages", chat.params.MaxTokens, tokenCount)
		}
	}
	numTokens := tokenCount + ChatPrimedTokens
	for {
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
		if (numTokens + parentMessage.TokenCount) >= (chat.params.MaxTokens - chat.params.ChatMinResponseTokens) {
			break
		}
		numTokens += parentMessage.TokenCount
		messages = append(messages, parentCompletioMessage)
		parentMessageId = parentMessage.ParentMessageId
	}
	utils.Reverse(messages)
	return messages, numTokens, tokenCount, nil
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
