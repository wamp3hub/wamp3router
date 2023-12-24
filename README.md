# WAMP3Router

It implements the open
[Web Application Messaging Protocol (WAMP)](https://wamp-proto.org/index.html).

![WAMP Router](https://mediacomem.github.io/comem-archioweb/2021-2022/subjects/wamp/images/routed-protocol.png)

## Web Application Messaging Protocol

WAMP is an open application level protocol registered at
[IANA](https://www.iana.org/assignments/websocket/websocket.xml)
that provides two messaging patterns:

* [Routed Remote Procedure Call (RPC)](https://wamp-proto.org/faq.html#what-is-rpc)
* [Publish & Subscribe (PubSub)](https://wamp-proto.org/faq.html#what-is-pubsub)

## Installation

```bash
git clone git@github.com:wamp3hub/wamp3router.git
cd ./source/daemon/
go build -o wamp3rd
./wamp3rd
```

## Docker

build
```bash
docker build -t wamp3rd:latest .
```

run
```
docker run -p 8800:8800 --name wamp3rd -d wamp3rd
```

## Roadmap

- Scalability
- Security
- Realm Gateway
