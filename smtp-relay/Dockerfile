# UniMap webhook -> SMTP email relay.
# Python 3 slim keeps the image tiny; only the standard library is used.
FROM python:3-slim
WORKDIR /app
COPY relay.py /app/relay.py
EXPOSE 8099
CMD ["python", "-u", "/app/relay.py"]
