FROM python:3.10-alpine

WORKDIR app

ADD tokenizer.py /app/tokenizer.py
ADD requirements.txt /app/requirements.txt
RUN pip install -i https://mirrors.aliyun.com/pypi/simple --upgrade pip
RUN pip install --root-user-action=ignore -i https://mirrors.aliyun.com/pypi/simple -r requirements.txt