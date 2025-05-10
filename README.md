# LogIt-Go

LogIt-Go - это мощная и гибкая библиотека для логирования в Go-приложениях, построенная на основе [Zap](https://github.com/uber-go/zap) и интегрированная с [Sentry](https://sentry.io/).

## Особенности

- Высокопроизводительное логирование с использованием Zap
- Поддержка ротации логов
- Интеграция с Sentry для отслеживания ошибок
- Настраиваемые уровни логирования для консоли и файла
- Автоматическое генерирование ID трассировки
- Поддержка структурированного логирования

## Установка

```bash
go get github.com/x3a-tech/logit-go
```

## Использование
### Инициализация
```go
import (
    "github.com/x3a-tech/logit-go"
    "github.com/x3a-tech/configo"
    "github.com/x3a-tech/envo"
)

func main() {
    const op := "main"
    appConf := &configo.App{...}
    loggerConf := &configo.Logger{...}
    senConf := &configo.Sentry{...}
    env := envo.New()

    logger := logit.MustNewLogger(appConf, loggerConf, senConf, env)

// Использование логгера
    ctx := logger.NewTraceCtx(nil, &op)
    logger.Info(ctx, "Приложение успешно запущено")
```

### Логирование
```go
const op := "module.submodule.Method"
ctx := logger.NewCtx(nil, &op, nil)
logger.Debug("Debug message")
logger.Info(ctx, "Info message")
logger.Warn(ctx, "Warning message")
logger.Error(ctx, err)
logger.Fatal(ctx, err)
```

### Тестирование

Для использования в тестах предусмотрен пустой логгер

```go
logger := logit.NewNopLogger()
```

### Конфигурация логгера

Логгер принимает следующие параметры конфигурации:

#### App конфигурация (configo.App):
- `Name`: Имя приложения
- `Version`: Версия приложения

#### Logger конфигурация (configo.Logger):
- `EnableConsole`: Включить вывод в консоль (bool)
- `ConsoleLevel`: Уровень логирования для консоли (int)
- `EnableFile`: Включить запись в файл (bool)
- `FileLevel`: Уровень логирования для файла (int)
- `Dir`: Директория для хранения лог-файлов
- `MaxSize`: Максимальный размер файла лога в мегабайтах
- `MaxBackups`: Максимальное количество старых лог-файлов для хранения
- `MaxAge`: Максимальное время хранения старых лог-файлов в днях
- `Compress`: Сжимать ротированные лог-файлы (bool)
- `TimeFormat`: Формат времени для логов
- `RotationTime`: Интервал ротации логов (например, "24h")

#### Sentry конфигурация (configo.Sentry):
- `Key`: Ключ проекта Sentry
- `Host`: Хост Sentry

#### Env конфигурация (envo.Env):
- Объект, представляющий текущее окружение

## Использование
### Инициализация 
```go
import (
    "context"
    "https://github.com/x3a-tech/logit-go"
    "https://github.com/x3a-tech/configo"
    "go.uber.org/zap" // Для zap.Field, если нужно
)

func main() {
    // Базовый контекст
    ctx := context.Background()

    // Пример конфигурации (заполните своими значениями)
    appConf := &configo.App{
        Name:    "MyApp",
        Version: "1.0.0",
    }
    loggerConf := &configo.Logger{
        EnableConsole: true,
        ConsoleLevel:  int(zapcore.DebugLevel), // или используйте числовые значения
        EnableFile:    true,
        FileLevel:     int(zapcore.InfoLevel),
        Dir:           "logs",
        MaxSize:       100,
        MaxBackups:    3,
        MaxAge:        28,
        Compress:      true,
        TimeFormat:    "2006-01-02 15:04:05",
        RotationTime:  "24h", // Например, "24h", "1h30m". Убедитесь, что формат корректен.
    }
    senConf := &configo.Sentry{ // Опционально
        Key:  "your-sentry-key",
        Host: "your-sentry-host.example.com", // Например, oXXXXXX.ingest.sentry.io (без https://)
    }

    // Инициализация логгера
    loggerParams := &logit.Params{
        AppConf:    appConf,
        LoggerConf: loggerConf,
        SenConf:    senConf, // может быть nil, если Sentry не используется
        Env:        env,
    }
    logger := logit.MustNewLogger(loggerParams)

    // Установка операции и traceId в контекст
    // Устанавливаем имя операции
    ctx = logger.NewOpCtx(ctx, "main.startup")

    // Если traceId уже есть (например, из входящего запроса):
    // incomingTraceId := "abc-123-xyz-789"
    // ctx = logger.NewTraceCtx(ctx, &incomingTraceId)
    // Если traceId нужно сгенерировать:
    ctx = logger.NewTraceCtx(ctx, nil) // nil сгенерирует новый traceId

    // Использование логгера
    logger.Info(ctx, "Приложение успешно запущено", zap.String("component", "main"))
    logger.Debug(ctx, "Это отладочное сообщение с дополнительными полями", zap.Int("userID", 123))
}
```

## Config

```yaml
app:
  name: "MyApp"
  version: "1.0.0"

logger:
  level: 0
  dir: "logs"
  maxSize: 10
  maxBackups: 3
  maxAge: 365
  compress: true
  rotationTime: "24h"
  consoleLevel: 0
  fileLevel: 0
  enableConsole: true
  enableFile: true
  timeFormat: "2006-01-02T15:04:05.000Z07:00"
```


### Описание полей
#### App конфигурация

| Параметр | Описание |
|----------|----------|
| `name`   | Имя приложения |
| `version`| Версия приложения |

#### Logger конфигурация

| Параметр | Описание | Тип | Значение по умолчанию |
|----------|----------|-----|------------------------|
| `level` | Общий уровень логирования | int | 0 |
| `dir` | Директория для хранения лог-файлов | string | "logs" |
| `maxSize` | Максимальный размер файла лога в мегабайтах | int | 10 |
| `maxBackups` | Максимальное количество старых лог-файлов для хранения | int | 3 |
| `maxAge` | Максимальное время хранения старых лог-файлов в днях | int | 365 |
| `compress` | Сжимать ротированные лог-файлы | bool | true |
| `rotationTime` | Интервал ротации логов | string | "24h" |
| `consoleLevel` | Уровень логирования для консоли | int | 0 |
| `fileLevel` | Уровень логирования для файла | int | 0 |
| `enableConsole` | Включить вывод в консоль | bool | true |
| `enableFile` | Включить запись в файл | bool | true |
| `timeFormat` | Формат времени для логов | string | "2006-01-02T15:04:05.000Z07:00" |