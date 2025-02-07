FROM chenzhaoyu94/chatgpt-web:v2.10.9 as frontend

FROM arvintian/chatgpt-web-base:v5

COPY --from=frontend /app/public /app/public

COPY web/admin /app/public/admin

COPY web/static /app/public/static

ADD dist/server /app/server
ADD tokenizer.py /app/tokenizer.py

RUN mkdir -p /data

EXPOSE 7080

CMD ["/app/server"]