

# gRPC‑сервис Pub/Sub

Небольшой микросервис на Go, реализующий **опубликование событий по ключу** и **потоковую подписку** на них через gRPC.  

---

## Возможности

* gRPC‑API: `Publish` (unary) и `Subscribe` (server‑stream)
* Корректное завершение (graceful shutdown) по `SIGINT` / `SIGTERM`
* Настройка через YAML‑файл

---

## Protobuf

```proto
syntax = "proto3";
package pubsub;
option go_package = "vkintern/pb;pubsub";

import "google/protobuf/empty.proto";

service PubSub {
  rpc Subscribe (SubscribeRequest) returns (stream Event);
  rpc Publish   (PublishRequest)   returns (google.protobuf.Empty);
}

message SubscribeRequest { string key  = 1; }
message PublishRequest   { string key  = 1; string data = 2; }
message Event            { string data = 1; }
```

Код генерируется командой:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       service.proto
```

Файлы `*.pb.go` уже находятся в репозитории, поэтому `protoc` запускать не обязательно.

---

## Сборка и запуск

```bash
go mod tidy      # подтянуть зависимости
go run .         # сервер запустится на :50051
```

Порт и таймауты можно изменить через `config.yaml` или переменные окружения (см. ниже).

---

## Конфигурация

Файл `config.yaml` (значения по умолчанию):

```yaml
grpc_port: 50051        # Порт для gRPC
shutdown_timeout: 5s    # Таймаут корректного завершения
```

---

## Быстрый тест через *grpcurl*

### Публикация

```bash
grpcurl -plaintext -d '{"key":"news","data":"hello"}' \
  localhost:50051 pubsub.PubSub/Publish
```

### Подписка (стрим)

```bash
grpcurl -plaintext -d '{"key":"news"}' \
  localhost:50051 pubsub.PubSub/Subscribe
```

### Отписка от стрима

Подписка завершается, когда клиент закрывает соединение.  
В `grpcurl` достаточно нажать `Ctrl‑C` в терминале с активным стримом — сервис зафиксирует событие, вызовет `Unsubscribe()` во внутреннем брокере и запишет `subscription closed` в лог.
## Завершение работы сервиса

Сервер перехватывает сигналы `SIGINT` и `SIGTERM`.  
При получении сигнала выполняются шаги:

1. вызывается `grpcServer.GracefulStop()`– новые соединения не принимаются, активные RPC получают время завершиться;
2. внутренний брокер `subpub` закрывается через `broker.Close(ctx)`, где `ctx` ограничен `shutdown_timeout` из `config.yaml`;
3. по истечении таймаута (или раньше, если все потоки завершились) процесс завершается.

---

## Использованные паттерны

| Паттерн | Назначение | Где в коде |
|---------|------------|------------|
| **Dependency Injection (конструкторы)** | Явное связывание и удобство тестирования | `main.go`, `NewServer`, `NewLogger` |
| **Factory / Constructor** | Инкапсулируют создание объектов | `subpub.NewSubPub()`, др. |
| **Graceful Shutdown** | Корректная остановка сервиса | `main.go` |
| **Externalized Configuration** | Настройка без перекомпиляции | `config.go`, `config.yaml` |
| **Context cancellation** | Передача таймаутов / отмены | везде (`Publish`, `Subscribe`, `Close`) |