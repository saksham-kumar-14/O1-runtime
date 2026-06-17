FROM alpine
RUN apk update
RUN apk add python3
CMD ["python3", "-c", "print('Hello from my custom O1 image!')"]
