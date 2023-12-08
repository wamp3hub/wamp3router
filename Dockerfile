FROM golang:1.21

ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /wamp3rd

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN go build -o /docker-wamp3rd

CMD ["/docker-wamp3rd"]
