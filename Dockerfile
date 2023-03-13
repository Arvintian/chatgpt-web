FROM chenzhaoyu94/chatgpt-web:v2.10.5 as frontend

FROM python:3.10-alpine

COPY --from=frontend /app/public /app/public

WORKDIR app

ADD tokenizer.py /app/tokenizer.py
ADD requirements.txt /app/requirements.txt
RUN pip install -i https://mirrors.aliyun.com/pypi/simple --upgrade pip
RUN pip install --root-user-action=ignore -i https://mirrors.aliyun.com/pypi/simple -r requirements.txt
ADD dist/server /app/server

EXPOSE 7080

CMD ["/app/server"]