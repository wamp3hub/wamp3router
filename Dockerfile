FROM golang:1.21

ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /wamp3rd

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -C source/daemon -o wamp3rd

EXPOSE 8800

CMD ./source/daemon/wamp3rd run
