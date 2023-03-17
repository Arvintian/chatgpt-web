Built ChatGPT service using [Gin](https://github.com/gin-gonic/gin) and [ChatGPT Web](https://github.com/Chanzhaoyu/chatgpt-web) as the front-end. 

## Usage

[Docker Hub](https://hub.docker.com/repository/docker/arvintian/chatgpt-web/general)

```
docker run --restart unless-stopped -d --log-opt max-size=50m -p 7080:7080 \
-e OPENAI_KEY=openai-key \
-e BASIC_AUTH_USER=user1,user2 \
-e BASIC_AUTH_PASSWORD=passwd1,passwd2 \
arvintian/chatgpt-web
```

- SERVER_PORT: Server port, default 7080.
- SERVER_HOST: Server listen address, default 0.0.0.0.
- SOCKS_PROXY: Socks proxy URL, for example socks5://user:password@127.0.0.1:1080.
- BASIC_AUTH_USER: Authentication user, multiple users separated by English commas.
- BASIC_AUTH_PASSWORD: Authentication user passwords, multiple users separated by English commas.
- CHAT_SESSION_TTL: Session context retention time, default 30 minutes.
- CHAT_MIN_RESPONSE_TOKENS: Tokens reserved for session response, may lead to truncation of the longest context, default 600.
- OPENAI_KEY: OpenAI API key, refer to OpenAI documentation.
- OPENAI_BASE_URL: OpenAI API base URL, default https://api.openai.com/v1.
- OPENAI_MODEL: Model called, default gpt-3.5-turbo-0301.
- OPENAI_MAX_TOKENS: Model max_tokens parameter, refer to OpenAI documentation.
- OPENAI_TEMPERATURE: Model temperature parameter, refer to OpenAI documentation.
- OPENAI_PRESENCE_PENALTY: Model presence_penalty parameter, refer to OpenAI documentation.
- OPENAI_FREQUENCY_PENALTY: Model frequency_penalty parameter, refer to OpenAI documentation.

For more detailed parameters, please refer to the [start function](https://github.com/Arvintian/chatgpt-web/blob/main/cmd/main.go#L21).

Tips: 
- Use (integer/100) to set the float32 model parameters. For example, if temperature is set to 0.8, it needs to be set to 80.
- The built-in support for a forward proxy of OPENAI_BASE_URL enables it to function as a proxy server for the OpenAI API.