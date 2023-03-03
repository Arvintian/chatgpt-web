FROM arvintian/chatgpt-vue:f95af40

ADD dist/server /app/server

ENTRYPOINT ["/app/server"]