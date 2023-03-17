# ChatGPT-Web

[English](https://github.com/Arvintian/chatgpt-web/blob/main/README_en.md)

使用[Gin](https://github.com/gin-gonic/gin)搭建ChatGPT服务,使用[ChatGPT Web](https://github.com/Chanzhaoyu/chatgpt-web)作为前端

## Usage

[Docker Hub](https://hub.docker.com/repository/docker/arvintian/chatgpt-web/general)

```
docker run --restart unless-stopped -d --log-opt max-size=50m -p 7080:7080 \
-e OPENAI_KEY=openai-key \
-e BASIC_AUTH_USER=user1,user2 \
-e BASIC_AUTH_PASSWORD=passwd1,passwd2 \
arvintian/chatgpt-web
```

- SERVER_PORT 服务端口,默认7080
- SERVER_HOST 服务监听地址,默认0.0.0.0
- SOCKS_PROXY socks代理URL,例如socks5://user:password@127.0.0.1:1080
- BASIC_AUTH_USER 认证用户,多用户英文逗号分隔
- BASIC_AUTH_PASSWORD 认证用户密码,多用户英文逗号分隔
- CHAT_SESSION_TTL 会话上下文保持时间,默认30分钟
- CHAT_MIN_RESPONSE_TOKENS 预留给会话响应的token数,可能导致截断最久的上下文,默认600
- OPENAI_KEY openai api key,参考OpenAI文档
- OPENAI_BASE_URL openai api base url,默认https://api.openai.com/v1
- OPENAI_MODEL 调用模型,默认gpt-3.5-turbo-0301
- OPENAI_MAX_TOKENS 模型max_tokens参数,参考OpenAI文档
- OPENAI_TEMPERATURE 模型temperature参数,参考OpenAI文档
- OPENAI_PRESENCE_PENALTY 模型presence_penalty参数,参考OpenAI文档
- OPENAI_FREQUENCY_PENALTY 模型frequency_penalty参数,参考OpenAI文档

更详细参数参考: [启动函数](https://github.com/Arvintian/chatgpt-web/blob/main/cmd/main.go#L21)

Tips: 
- 模型float32参数使用(整型/100)设置,例如: temperature设置0.8,需要设置为80
- 内置支持了对OPENAI_BASE_URL的正向代理,可以作为OpenAI接口的代理服务器