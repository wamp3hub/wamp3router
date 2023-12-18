FROM golang:1.21

ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /wamp3rd

COPY go.mod go.sum ./

RUN go mod download

COPY ./source/ ./

RUN go build -C daemon -o wamp3rd

CMD ./daemon/wamp3rd run
