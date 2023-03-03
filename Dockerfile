FROM chenzhaoyu94/chatgpt-web:v2.9.1

ADD dist/server /app/server

ENTRYPOINT ["/app/server"]